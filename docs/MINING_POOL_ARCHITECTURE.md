# OpenSY Mining Pool Architecture

> **Status**: Planning Document  
> **Priority**: High  
> **Complexity**: High

## Overview

This document outlines the architecture for an OpenSY mining pool supporting the RandomX proof-of-work algorithm.

## Architecture Components

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Mining Pool Architecture                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│  │   Miner 1    │    │   Miner 2    │    │   Miner N    │              │
│  │  (XMRig)     │    │  (XMRig)     │    │  (XMRig)     │              │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘              │
│         │                    │                    │                      │
│         └────────────────────┼────────────────────┘                      │
│                              │                                           │
│                     Stratum Protocol (TCP)                               │
│                              │                                           │
│                              ▼                                           │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                     STRATUM SERVER                                 │  │
│  │  • Job distribution       • Share validation                      │  │
│  │  • Difficulty adjustment  • Miner authentication                  │  │
│  │  • Connection handling    • Rate limiting                         │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                              │                                           │
│                              ▼                                           │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                      BLOCK TEMPLATE ENGINE                         │  │
│  │  • getblocktemplate RPC   • Transaction selection                 │  │
│  │  • Merkle tree building   • Block header construction             │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                              │                                           │
│                              ▼                                           │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                       OPENSY NODE                                  │  │
│  │  • Block validation       • Transaction relay                     │  │
│  │  • RandomX verification   • P2P network                           │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                      DATABASE (Redis + PostgreSQL)                 │  │
│  │  • Share tracking         • Worker stats                          │  │
│  │  • Block history          • Payout records                        │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                      PAYOUT ENGINE                                 │  │
│  │  • PPLNS/PPS calculation  • Balance tracking                      │  │
│  │  • Automated payouts      • Fee deduction                         │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                      WEB DASHBOARD                                 │  │
│  │  • Miner stats            • Pool hashrate                         │  │
│  │  • Payout history         • Block history                         │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Technology Stack

| Component | Technology | Reason |
|-----------|------------|--------|
| Stratum Server | Node.js / Go | High concurrency, low latency |
| Database | Redis + PostgreSQL | Fast caching + reliable persistence |
| Web Dashboard | React / Vue.js | Modern, responsive UI |
| API Server | Flask / FastAPI | Python ecosystem for pool logic |
| Message Queue | Redis Pub/Sub | Real-time updates |
| Monitoring | Prometheus + Grafana | Industry standard |

## Stratum Protocol

### Connection Flow

```
Miner                                   Pool
  │                                       │
  │───── TCP Connect ────────────────────>│
  │                                       │
  │<──── {"jsonrpc":"2.0","method":"job"} │ (Initial job)
  │                                       │
  │───── {"method":"login",...} ─────────>│ (Auth)
  │                                       │
  │<──── {"status":"OK","job":{...}} ─────│
  │                                       │
  │ ... mining ...                        │
  │                                       │
  │───── {"method":"submit",...} ─────────>│ (Share)
  │                                       │
  │<──── {"status":"OK"} ─────────────────│
  │                                       │
```

### Job Message (Pool → Miner)

```json
{
  "jsonrpc": "2.0",
  "method": "job",
  "params": {
    "blob": "0707...hex...",
    "job_id": "abc123",
    "target": "b7d60000",
    "height": 10379,
    "seed_hash": "7f...hex..."
  }
}
```

### Submit Message (Miner → Pool)

```json
{
  "id": 1,
  "jsonrpc": "2.0",
  "method": "submit",
  "params": {
    "id": "worker1",
    "job_id": "abc123",
    "nonce": "deadbeef",
    "result": "0100...hash..."
  }
}
```

## RandomX Specifics

### Mining Context

The pool must maintain a RandomX mining context:

```cpp
// Pool needs full RandomX dataset for fast share validation
randomx_flags flags = randomx_get_flags();
flags |= RANDOMX_FLAG_FULL_MEM;  // ~2GB RAM
flags |= RANDOMX_FLAG_JIT;       // JIT compilation

randomx_cache* cache = randomx_alloc_cache(flags);
randomx_init_cache(cache, seed_hash, 32);

randomx_dataset* dataset = randomx_alloc_dataset(flags);
// Initialize dataset with multiple threads
randomx_init_dataset(dataset, cache, 0, count);

randomx_vm* vm = randomx_create_vm(flags, cache, dataset);
```

### Seed Hash Updates

**OpenSY uses a 32-block key interval** (not 2048 like Monero). This means the RandomX dataset must be regenerated approximately every 64 minutes.

```python
# OpenSY-specific: 32-block key interval (defined in consensus/params.h)
KEY_BLOCK_INTERVAL = 32

def get_seed_height(height):
    """Calculate seed block height for RandomX key.
    
    OpenSY rotates the RandomX key every 32 blocks for tighter security.
    The key block is the block whose hash initializes the RandomX cache/dataset.
    """
    if height < KEY_BLOCK_INTERVAL:
        return 0  # Use genesis for early blocks
    # Key block is KEY_BLOCK_INTERVAL blocks behind, rounded to interval
    return ((height // KEY_BLOCK_INTERVAL) - 1) * KEY_BLOCK_INTERVAL

def needs_new_seed(old_height, new_height):
    """Check if we need to recalculate dataset."""
    return get_seed_height(old_height) != get_seed_height(new_height)
```

### Share Difficulty

Adjust miner difficulty based on submission rate:

```python
TARGET_SHARE_TIME = 30  # seconds between shares

def calculate_new_difficulty(current_diff, avg_share_time):
    """Variable difficulty algorithm."""
    ratio = TARGET_SHARE_TIME / avg_share_time
    # Clamp adjustment to prevent wild swings
    ratio = max(0.5, min(2.0, ratio))
    new_diff = current_diff * ratio
    # Minimum and maximum bounds
    return max(1000, min(new_diff, NETWORK_DIFFICULTY))
```

## Payout Schemes

### PPLNS (Pay Per Last N Shares)

```python
def calculate_pplns_payout(block_reward, shares, N=10000):
    """
    PPLNS: Pay based on shares in last N shares window.
    More resistant to pool hopping.
    """
    last_n_shares = shares[-N:]
    total_diff = sum(s.difficulty for s in last_n_shares)
    
    payouts = {}
    for share in last_n_shares:
        worker = share.worker
        proportion = share.difficulty / total_diff
        payouts[worker] = payouts.get(worker, 0) + (block_reward * proportion)
    
    return payouts
```

### PPS (Pay Per Share)

```python
def calculate_pps_payout(share_difficulty, network_difficulty, block_reward):
    """
    PPS: Fixed payout per share regardless of blocks found.
    Higher risk for pool operator.
    """
    expected_shares = network_difficulty / share_difficulty
    payment_per_share = block_reward / expected_shares
    return payment_per_share
```

### PROP (Proportional)

```python
def calculate_prop_payout(block_reward, shares_since_last_block):
    """
    PROP: Proportional shares since last block.
    Vulnerable to pool hopping.
    """
    total_diff = sum(s.difficulty for s in shares_since_last_block)
    
    payouts = {}
    for share in shares_since_last_block:
        proportion = share.difficulty / total_diff
        payouts[share.worker] = payouts.get(share.worker, 0) + (block_reward * proportion)
    
    return payouts
```

## Database Schema

```sql
-- Workers table
CREATE TABLE workers (
    id SERIAL PRIMARY KEY,
    address VARCHAR(64) NOT NULL,
    worker_name VARCHAR(64),
    first_seen TIMESTAMP DEFAULT NOW(),
    last_seen TIMESTAMP,
    total_shares BIGINT DEFAULT 0,
    valid_shares BIGINT DEFAULT 0,
    invalid_shares BIGINT DEFAULT 0
);

CREATE INDEX idx_workers_address ON workers(address);

-- Shares table
CREATE TABLE shares (
    id BIGSERIAL PRIMARY KEY,
    worker_id INTEGER REFERENCES workers(id),
    difficulty BIGINT NOT NULL,
    share_diff BIGINT NOT NULL,
    height INTEGER NOT NULL,
    timestamp TIMESTAMP DEFAULT NOW(),
    is_valid BOOLEAN DEFAULT true,
    is_block BOOLEAN DEFAULT false
);

CREATE INDEX idx_shares_timestamp ON shares(timestamp);
CREATE INDEX idx_shares_worker ON shares(worker_id);

-- Blocks table
CREATE TABLE blocks (
    id SERIAL PRIMARY KEY,
    height INTEGER NOT NULL UNIQUE,
    hash VARCHAR(64) NOT NULL,
    reward DECIMAL(20, 8) NOT NULL,
    finder_worker_id INTEGER REFERENCES workers(id),
    timestamp TIMESTAMP DEFAULT NOW(),
    confirmations INTEGER DEFAULT 0,
    orphaned BOOLEAN DEFAULT false
);

-- Payouts table
CREATE TABLE payouts (
    id SERIAL PRIMARY KEY,
    worker_id INTEGER REFERENCES workers(id),
    amount DECIMAL(20, 8) NOT NULL,
    txid VARCHAR(64),
    timestamp TIMESTAMP DEFAULT NOW(),
    status VARCHAR(20) DEFAULT 'pending'
);

-- Pool stats table
CREATE TABLE pool_stats (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT NOW(),
    hashrate BIGINT,
    workers_online INTEGER,
    blocks_found INTEGER,
    network_difficulty DECIMAL(30, 10)
);
```

## API Endpoints

### Public Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/pool/stats` | Pool hashrate, blocks, workers |
| `GET /api/pool/blocks` | Recent blocks found |
| `GET /api/network/stats` | Network info |
| `GET /api/miner/{address}/stats` | Miner statistics |
| `GET /api/miner/{address}/payments` | Payout history |
| `GET /api/miner/{address}/workers` | Worker list |

### Example Responses

```json
// GET /api/pool/stats
{
  "hashrate": 5000000,
  "hashrate_formatted": "5.00 MH/s",
  "workers_online": 150,
  "blocks_found_24h": 12,
  "blocks_total": 500,
  "pool_fee": 1.0,
  "payout_scheme": "PPLNS",
  "min_payout": 100,
  "payout_interval": "hourly"
}
```

```json
// GET /api/miner/{address}/stats
{
  "address": "syl1q...",
  "hashrate": 3000,
  "hashrate_formatted": "3.00 KH/s",
  "shares_valid": 15000,
  "shares_invalid": 12,
  "balance": 5420.5,
  "paid_total": 25000,
  "last_share": "2025-01-15T12:00:00Z",
  "workers": [
    {
      "name": "rig1",
      "hashrate": 1500,
      "last_share": "2025-01-15T12:00:00Z"
    },
    {
      "name": "rig2", 
      "hashrate": 1500,
      "last_share": "2025-01-15T11:59:30Z"
    }
  ]
}
```

## Security Considerations

### DDoS Protection

1. **Rate limiting** per IP and per wallet
2. **Connection limits** per IP
3. **Proof-of-work** for login (optional)
4. **Cloudflare** or similar CDN for web dashboard

### Share Fraud Prevention

1. **Duplicate share detection** using Redis sets
2. **Job ID validation** with expiration
3. **Nonce validation** (4-byte range per job)
4. **Result hash verification**

### Wallet Security

```python
# Pool wallet configuration
POOL_WALLET = {
    'cold_wallet': 'syl1q...cold...',  # 90% of funds
    'hot_wallet': 'syl1q...hot...',     # 10% for payouts
    'hot_wallet_max': 100000,            # Max SYL in hot wallet
}
```

## Deployment

### Infrastructure Requirements

| Component | Min Specs | Recommended |
|-----------|-----------|-------------|
| Stratum Server | 4 CPU, 8GB RAM | 8 CPU, 16GB RAM |
| Pool Node | 4 CPU, 8GB RAM | 8 CPU, 16GB RAM |
| Database | 4 CPU, 8GB RAM, SSD | 8 CPU, 32GB RAM, NVMe |
| Web Server | 2 CPU, 4GB RAM | 4 CPU, 8GB RAM |

### Docker Compose

```yaml
version: '3.8'

services:
  opensy-node:
    image: opensy/node:latest
    volumes:
      - node-data:/data
    ports:
      - "9632:9632"  # RPC
      - "9633:9633"  # P2P
    command: opensyd -datadir=/data -rpcallowip=172.0.0.0/8

  stratum:
    build: ./stratum
    ports:
      - "3333:3333"  # Stratum port
    depends_on:
      - opensy-node
      - redis
      - postgres
    environment:
      - NODE_RPC=http://opensy-node:9632
      - REDIS_URL=redis://redis:6379
      - DATABASE_URL=postgres://pool:password@postgres/pool

  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data

  postgres:
    image: postgres:15
    volumes:
      - postgres-data:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=pool
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=pool

  web:
    build: ./web-dashboard
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - stratum

volumes:
  node-data:
  redis-data:
  postgres-data:
```

## Implementation Phases

### Phase 1: Core Infrastructure (4-6 weeks)
- [ ] Basic stratum server with job distribution
- [ ] Share validation with RandomX
- [ ] Single-worker support
- [ ] PostgreSQL integration

### Phase 2: Multi-Worker & Dashboard (2-4 weeks)
- [ ] Multi-worker per address support
- [ ] Variable difficulty
- [ ] Basic web dashboard
- [ ] Real-time stats via WebSocket

### Phase 3: Payouts (2-3 weeks)
- [ ] PPLNS implementation
- [ ] Balance tracking
- [ ] Automated payouts
- [ ] Payout history

### Phase 4: Production Hardening (2-4 weeks)
- [ ] DDoS protection
- [ ] Monitoring & alerting
- [ ] Load testing
- [ ] Security audit

## Compatible Mining Software

| Software | Platform | Notes |
|----------|----------|-------|
| XMRig | All | Best RandomX miner, requires `algo: "rx/0"` |
| xmr-stak-rx | All | Alternative |
| SRBMiner-MULTI | Windows/Linux | Multi-algo |

> **Note**: OpenSY uses standard RandomX (rx/0) but with a **32-block key interval** instead of Monero's 2048. Pool software must handle more frequent dataset regeneration (~every 64 minutes vs ~2.8 days).

### XMRig Configuration

```json
{
  "autosave": true,
  "cpu": true,
  "opencl": false,
  "cuda": false,
  "pools": [
    {
      "url": "pool.opensyria.net:3333",
      "user": "syl1qvg2uuau5xegn0nt8fly5m2xm84uvgn3m3aermx",
      "pass": "worker1",
      "algo": "rx/0"
    }
  ]
}
```

## Monitoring

### Prometheus Metrics

```
# Pool metrics
pool_hashrate_hps gauge
pool_workers_online gauge
pool_shares_total counter
pool_shares_invalid counter
pool_blocks_found counter
pool_blocks_orphaned counter

# Per-worker metrics
worker_hashrate_hps{address="syl1..."} gauge
worker_shares_valid{address="syl1..."} counter
```

### Alerting Rules

```yaml
groups:
  - name: pool-alerts
    rules:
      - alert: PoolHashrateDrop
        expr: pool_hashrate_hps < 1000000
        for: 10m
        annotations:
          summary: Pool hashrate dropped below 1 MH/s

      - alert: StratumServerDown
        expr: up{job="stratum"} == 0
        for: 1m
        annotations:
          summary: Stratum server is down

      - alert: NoBlocksFound
        expr: increase(pool_blocks_found[1h]) == 0
        for: 2h
        annotations:
          summary: No blocks found in 2 hours
```

## Conclusion

This architecture provides a scalable, secure mining pool for OpenSY. The RandomX-specific optimizations ensure efficient share validation while maintaining compatibility with existing mining software like XMRig.

### Key Decisions

1. **Full RandomX dataset** for fast share validation (~1ms vs ~100ms)
2. **PPLNS payout** to discourage pool hopping
3. **Variable difficulty** to optimize share rate
4. **Redis + PostgreSQL** for speed and reliability
5. **32-block key interval** - Pool must regenerate dataset every ~64 minutes (OpenSY-specific)

### OpenSY-Specific Parameters

| Parameter | Value | Notes |
|-----------|-------|-------|
| Block time | 2 minutes | Faster than Monero (2 min vs 2 min) |
| Key interval | 32 blocks | Much faster than Monero (2048) |
| Dataset regen | ~64 minutes | Plan for ~30-60s regeneration time |
| Block reward | 10,000 SYL | Initial, halves over time |
| Coinbase maturity | 100 blocks | ~3.3 hours to spend rewards |

### Next Steps

1. Begin Phase 1 development
2. Set up development environment
3. Implement basic stratum server
4. Test with XMRig

---

*Document Version: 1.0*  
*Last Updated: 2025-01-15*

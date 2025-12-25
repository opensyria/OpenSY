# OpenSY Mining Infrastructure

> **Status**: Development  
> **Language**: Go 1.21+  
> **License**: MIT

Complete mining infrastructure for OpenSY blockchain, featuring:

- **Mining Pool**: Traditional Stratum-based pool with PPLNS payouts
- **CoopMine**: Cooperative hashrate aggregation cluster
- **Common**: Shared libraries (RandomX CGO, RPC client)

## Features

### Mining Pool
- ✅ **Stratum v1 Protocol** - Compatible with XMRig and other RandomX miners
- ✅ **RandomX CGO Bindings** - Tested on x86 and ARM
- ✅ **PPLNS Payouts** - Fair reward distribution
- ✅ **Variable Difficulty** - Auto-adjusting miner difficulty
- ✅ **REST API** - Dashboard and miner stats
- ✅ **PostgreSQL** - Partitioned tables for high-volume shares
- ✅ **Redis** - Caching, hashrate calculation, deduplication
- ✅ **Prometheus Metrics** - Monitoring ready

### CoopMine Cluster
- ✅ **Coordinator Node** - Central job distribution and share aggregation
- ✅ **Worker Nodes** - Distributed mining across multiple machines
- ✅ **gRPC Protocol** - Low-latency coordinator-worker communication
- ✅ **Pool Client** - Upstream connection to any Stratum pool
- ✅ **Dynamic Work Assignment** - Unique extra nonce per worker
- ✅ **Health Monitoring** - Automatic worker timeout detection
- ✅ **Stats Aggregation** - Unified cluster hashrate reporting
- ✅ **Share Forwarding** - Automatic submission to upstream pool

### Production Security Features
- ✅ **Rate Limiting** - Per-IP request throttling
- ✅ **IP Banning** - Automatic and manual ban management
- ✅ **JWT Authentication** - Secure API access
- ✅ **Circuit Breaker** - RPC failure protection
- ✅ **Input Validation** - Comprehensive sanitization
- ✅ **Health Endpoints** - `/health`, `/ready`, `/live`
- ✅ **Graceful Shutdown** - Clean connection termination

## Architecture

```
opensy-mining/
├── common/                    # Shared libraries
│   ├── randomx/              # RandomX CGO bindings (tested on AWS x86+ARM)
│   └── rpc/                  # OpenSY node JSON-RPC client
├── pool/                      # Mining pool server
│   ├── cmd/server/           # Main entry point
│   ├── stratum/              # Stratum protocol implementation
│   │   ├── protocol.go       # JSON-RPC types, difficulty math
│   │   ├── session.go        # Miner session state
│   │   ├── server.go         # TCP server, vardiff
│   │   └── job_manager.go    # Block templates, share validation
│   ├── db/                   # PostgreSQL layer
│   ├── cache/                # Redis caching
│   ├── payout/               # PPLNS reward distribution
│   ├── api/                  # REST API server
│   └── configs/              # Configuration files
├── coopmine/                  # CoopMine cluster system
│   ├── cmd/
│   │   ├── coordinator/      # Coordinator entry point
│   │   └── worker/           # Worker entry point
│   ├── coordinator.go        # Cluster coordinator logic
│   ├── worker.go             # Mining worker logic
│   ├── pool_client.go        # Upstream pool connection
│   ├── grpc_server.go        # Coordinator gRPC server
│   ├── grpc_client.go        # Worker gRPC client
│   ├── service.go            # Integrated service
│   └── proto/                # Protocol definitions
├── docker/                    # Docker configurations
│   ├── docker-compose.yml    # PostgreSQL + Redis + Prometheus
│   ├── init-db.sql           # Database schema
│   └── prometheus.yml        # Metrics config
└── scripts/                   # Build and test scripts
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- GCC (for CGO/RandomX)
- 4GB+ RAM (RandomX requires ~2GB)

### 1. Start Development Environment

```bash
# Start PostgreSQL + Redis + Prometheus
cd docker
docker-compose up -d

# Verify services
docker-compose ps
```

### 2. Build & Run

```bash
cd common
make randomx  # Downloads and builds RandomX C library
go test ./...
```

### 3. Run Pool (Development Mode)

```bash
cd pool
go run ./cmd/pool --config configs/dev.yaml
```

### 4. Test with XMRig

```bash
# Point XMRig to local pool
xmrig -o 127.0.0.1:3333 -u syl1testaddress -p worker1 -a rx/0
```

## CoopMine Usage

CoopMine enables multiple machines to mine cooperatively as a single unified miner.

### Quick Start Script

```bash
# Make script executable
chmod +x scripts/run-coopmine.sh

# On main machine (coordinator)
WALLET=SYxxxxxxxxx ./scripts/run-coopmine.sh coordinator

# On other machines (workers)
COORDINATOR=192.168.1.100:5555 ./scripts/run-coopmine.sh worker
```

### Manual Start - Coordinator

```bash
# The coordinator connects to an upstream pool and distributes work to workers
go run ./coopmine/cmd/coordinator \
  --wallet syl1yourwalletaddress \
  --pool pool.opensy.network:3333 \
  --grpc-addr :5555 \
  --cluster-name "My Mining Farm"
```

### Start Workers

```bash
# Workers connect to the coordinator and perform RandomX hashing
go run ./coopmine/cmd/worker \
  --coordinator coordinator-ip:5555 \
  --threads 4 \
  --worker-name "rig1"
```

### How It Works

```
┌─────────────┐
│  Mining     │  Jobs    ┌─────────────────────────────┐
│  Pool       │ ◄───────►│       COORDINATOR           │
│ (Stratum)   │  Shares  │  - Connects to pool         │
└─────────────┘          │  - Distributes jobs         │
                         │  - Aggregates shares        │
                         │  - Unique nonce per worker  │
                         └─────────────┬───────────────┘
                                       │ gRPC
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        ▼
      ┌───────────────┐        ┌───────────────┐        ┌───────────────┐
      │   WORKER 1    │        │   WORKER 2    │        │   WORKER N    │
      │   4 threads   │        │   8 threads   │        │   16 threads  │
      │   500 H/s     │        │   1000 H/s    │        │   2000 H/s    │
      └───────────────┘        └───────────────┘        └───────────────┘
```

The pool sees only one miner (the coordinator) with combined hashrate of all workers.

## OpenSY-Specific Parameters

| Parameter | Value | Notes |
|-----------|-------|-------|
| PoW Algorithm | RandomX (rx/0) | Standard RandomX |
| Block Time | 2 minutes | 120 seconds |
| Key Block Interval | 32 blocks | Dataset regen every ~64 min |
| Block Reward | 10,000 SYL | Initial, halves periodically |
| Coinbase Maturity | 100 blocks | ~3.3 hours |

## RPC Endpoints

| Network | RPC Port | P2P Port |
|---------|----------|----------|
| Mainnet | 9632 | 9633 |
| Testnet | 19632 | 19633 |
| Regtest | 18443 | 18444 |

## Documentation

- [Pool Operator Guide](docs/POOL_OPERATOR.md)
- [CoopMine Setup](docs/COOPMINE_SETUP.md)
- [RandomX Integration](docs/RANDOMX.md)
- [API Reference](docs/API.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## License

MIT License - see [LICENSE](LICENSE)

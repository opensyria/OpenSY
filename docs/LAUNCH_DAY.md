# 🇸🇾 OpenSyria Launch Day - December 8, 2025

> *"Dec 8 2024 - Syria Liberated from Assad / سوريا حرة"*
> — Genesis Block Message

## Network Launch Summary

**OpenSyria** officially launched on **December 8, 2025** — exactly one year after Syria's liberation.

### Network Details

| Parameter | Value |
|-----------|-------|
| **Launch Date** | December 8, 2025 |
| **Genesis Timestamp** | 1733616000 (Dec 8, 2024 00:00:00 UTC) |
| **Currency** | SYL (Syrian Lira) |
| **Initial Block Reward** | 10,000 SYL |
| **Block Time** | 2 minutes |
| **Max Supply** | 21,000,000,000 SYL |
| **Halving Interval** | 1,050,000 blocks (~4 years) |

### First Seed Node

| Component | Details |
|-----------|---------|
| **IP Address** | 157.175.40.131 |
| **Hostname** | node1.opensyria.net |
| **Region** | AWS Bahrain (me-south-1) |
| **P2P Port** | 9633 |
| **Version** | v30.99.0 |

### DNS Infrastructure

| Record | Target |
|--------|--------|
| `node1.opensyria.net` | 157.175.40.131 |
| `ns1.opensyria.net` | 157.175.40.131 |
| `seed.opensyria.net` | NS → ns1.opensyria.net |

### First Blocks

| Block | Hash | Timestamp |
|-------|------|-----------|
| Genesis (0) | `0000000727ee231c405685355f07629b06bfcb462cfa1ed7de868a6d9590ca8d` | Dec 8, 2024 |
| Block 1 | `0000001a06694a3c581ca3b3fad240676869336f04dccd42eccc72ffc988f526` | Dec 8, 2025 |

### First Address

```
syl1q0y76xxxdfvhfad2sju4fymnsn8zs5lndpwhufw
```

This bech32 address received the first block rewards.

## Connect to the Network

### Quick Start

```bash
# Clone and build
git clone https://github.com/opensyria/OpenSyria.git
cd OpenSyria
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)

# Run node
./build/bin/opensyriad -daemon

# Check status
./build/bin/opensyria-cli getblockchaininfo
```

### Connect to Seed Node

```bash
./build/bin/opensyriad -addnode=node1.opensyria.net
```

## Services Running

| Service | Status | Port |
|---------|--------|------|
| opensyriad | ✅ Active | 9633 (P2P), 9632 (RPC) |
| opensyria-seeder | ✅ Active | 53/UDP (DNS) |
| opensyria-miner | ✅ Active | N/A |

## Network Statistics (Live as of Launch Day)

| Metric | Value |
|--------|-------|
| **Blocks Mined** | 50,000+ |
| **Connected Peers** | 8+ nodes worldwide |
| **Network Hashrate** | ~350 MH/s |
| **Chain Size** | ~18 MB |

## How to Mine

### Solo Mining (CPU)

```bash
# Start your node
./build/bin/opensyriad -daemon -addnode=node1.opensyria.net

# Create a wallet
./build/bin/opensyria-cli createwallet "mining-wallet"

# Get a mining address
./build/bin/opensyria-cli -rpcwallet=mining-wallet getnewaddress "mining"

# Start mining (replace ADDRESS with your address)
./build/bin/opensyria-cli generatetoaddress 100 ADDRESS 500000000
```

### Continuous Mining

```bash
# Run in a loop
while true; do 
  ./build/bin/opensyria-cli generatetoaddress 10 YOUR_ADDRESS 500000000
done
```

> **Note:** Block rewards require 100 confirmations (~3.3 hours) before they can be spent.

## What's Next

- [ ] Deploy block explorer
- [ ] Create wallet releases (GUI)
- [ ] Add more seed nodes (geographic distribution)
- [ ] Community outreach
- [ ] Exchange listings

---

**سوريا حرة** 🇸🇾

*For the people of Syria, by the people of Syria.*

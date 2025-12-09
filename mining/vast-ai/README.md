# OpenSyria Mining on Vast.ai - Quick Start Guide

> **Last Updated:** December 9, 2025
> **Network Hashrate:** ~200 MH/s
> **Block Reward:** 500 SYL

## Step 1: Sign Up for Vast.ai
1. Go to https://vast.ai
2. Create account
3. Add credits ($50-100 recommended for serious mining)

## Step 2: Find a Machine

### Recommended GPUs (Best to Worst for SHA256d Mining):
| GPU | Est. Hashrate | Typical Price |
|-----|---------------|---------------|
| **RTX 5090** | ~200-250 MH/s | $0.40-0.55/hr |
| **RTX 4090** | ~100 MH/s | $0.30-0.45/hr |
| **RTX 3090** | ~50 MH/s | $0.20-0.30/hr |

### Top Picks (December 2025):
1. **4x RTX 5090 Texas** - ~800-1000 MH/s @ ~$2.20/hr (DOMINATION)
2. **2x RTX 5090 Hungary** - ~400-500 MH/s @ ~$0.98/hr (Strong)
3. **1x RTX 5090 Bulgaria** - ~200-250 MH/s @ ~$0.44/hr (Budget)

### Filter by:
- **GPU**: RTX 5090, RTX 4090, or RTX 3090
- **Price**: $0.30-2.50/hr depending on GPU count
- **Disk**: At least 50GB
- **Reliability**: 98%+ preferred

### Look for:
- Low price per hour
- High reliability score (99%+)
- Fast internet (1+ Gbps)
- PCIe 4.0/5.0 x16 for full bandwidth

## Step 3: Launch Instance

### Quick Setup Script (Recommended)

SSH into your Vast.ai instance and run this one-liner:

```bash
curl -sSL https://raw.githubusercontent.com/opensyria/OpenSyria/main/mining/vast-ai/setup.sh | bash
```

Or manually:

### Manual Setup

SSH into your Vast.ai instance and run:

```bash
# 1. Install dependencies
apt-get update && apt-get install -y git build-essential cmake libboost-all-dev \
  libevent-dev libssl-dev libsqlite3-dev jq screen

# 2. Clone and build OpenSyria
cd /root
git clone https://github.com/opensyria/OpenSyria.git
cd OpenSyria
cmake -B build -DBUILD_DAEMON=ON -DBUILD_CLI=ON -DBUILD_TESTS=OFF -DBUILD_GUI=OFF
cmake --build build -j$(nproc)

# 3. Create config
mkdir -p ~/.opensyria
cat > ~/.opensyria/opensyria.conf << 'EOF'
server=1
daemon=0
printtoconsole=1
addnode=157.175.40.131:9633
addnode=seed.opensyria.net:9633
rpcuser=miner
rpcpassword=minerpass123
rpcallowip=127.0.0.1
EOF

# 4. Start daemon in background
screen -dmS opensyriad ./build/bin/opensyriad -printtoconsole

# 5. Wait for sync (usually fast, ~1-2 minutes)
sleep 30
./build/bin/opensyria-cli getblockchaininfo

# 6. Start mining in a screen session
screen -dmS miner bash -c 'while true; do ./build/bin/opensyria-cli generatetoaddress 1 syl1q0y76xxxdfvhfad2sju4fymnsn8zs5lndpwhufw 500000000 2>/dev/null || sleep 5; done'

# 7. Monitor mining
screen -r miner  # Ctrl+A, D to detach
```

## Step 4: Monitor Mining

### Check mining status on Vast.ai instance:
```bash
# View miner output
screen -r miner

# Check block count
./build/bin/opensyria-cli getblockcount

# Check network hashrate
./build/bin/opensyria-cli getmininginfo
```

### Check balance on AWS main node:
```bash
ssh ubuntu@157.175.40.131
opensyria-cli -rpcwallet=founders-wallet getbalance
```

## Competition Analysis

Current competitor: `syl1qckjw8ardqpt6zcw9aylckxfw62swzq7gkjm794`
- Network hashrate: ~200 MH/s
- They're winning ~100% of blocks

### To Beat Them:
| Your Setup | Est. Hashrate | Win Rate |
|------------|---------------|----------|
| 1x RTX 5090 | ~200 MH/s | ~50% |
| 2x RTX 5090 | ~400 MH/s | ~67% |
| **4x RTX 5090** | ~800 MH/s | **~80%** |
| 8x RTX 5090 | ~1.6 GH/s | ~90% |

## Tips for Maximum Hashpower

1. **Rent more GPUs** - Each RTX 5090 adds ~200 MH/s
2. **Use RTX 5090 > 4090 > 3090** - Newer GPUs are much faster for SHA256d
3. **Pick high reliability hosts** - 99%+ uptime means consistent mining
4. **Monitor your balance** - Check `getbalance` to see blocks won

## Network Info

| Item | Value |
|------|-------|
| **Mining Address** | `syl1q0y76xxxdfvhfad2sju4fymnsn8zs5lndpwhufw` |
| **Seed Node** | `157.175.40.131:9633` |
| **DNS Seed** | `seed.opensyria.net` |
| **Block Reward** | 500 SYL |
| **Block Time** | ~10 minutes |
| **Current Difficulty** | 1 |

## Expected ROI

At current rates (December 2025):
- **4x RTX 5090** @ $2.22/hr = $53/day
- Winning 80% of blocks = ~72 blocks/day
- Earnings: 72 × 500 SYL = **36,000 SYL/day**

## Troubleshooting

### "Connection refused" error
```bash
# Restart daemon
pkill opensyriad
screen -dmS opensyriad ./build/bin/opensyriad -printtoconsole
sleep 30
```

### Not finding blocks
- Check you're synced: `getblockcount` should match network
- Check network hashrate: competitor may have upgraded
- Consider renting more GPUs

### Instance won't load
- High RAM instances (256GB+) take 30+ minutes to load
- Try lower RAM instances for faster startup
- Cancel stuck instances after 60+ minutes

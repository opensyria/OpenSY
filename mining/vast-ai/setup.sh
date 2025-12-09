#!/bin/bash
# OpenSyria GPU Mining Setup Script for Vast.ai
# Usage: curl -sSL https://raw.githubusercontent.com/opensyria/OpenSyria/main/mining/vast-ai/setup.sh | bash

set -e

# Configuration
MINING_ADDRESS="${MINING_ADDRESS:-syl1q0y76xxxdfvhfad2sju4fymnsn8zs5lndpwhufw}"
SEED_NODE="157.175.40.131:9633"
DNS_SEED="seed.opensyria.net:9633"

echo "=========================================="
echo "  OpenSyria GPU Mining Setup"
echo "=========================================="
echo "Mining Address: $MINING_ADDRESS"
echo ""

# 1. Install dependencies
echo "[1/6] Installing dependencies..."
apt-get update -qq
apt-get install -y -qq git build-essential cmake libboost-all-dev \
  libevent-dev libssl-dev libsqlite3-dev jq screen curl > /dev/null 2>&1
echo "✓ Dependencies installed"

# 2. Clone OpenSyria
echo "[2/6] Cloning OpenSyria..."
cd /root
if [ -d "OpenSyria" ]; then
    cd OpenSyria && git pull -q
else
    git clone -q https://github.com/opensyria/OpenSyria.git
    cd OpenSyria
fi
echo "✓ Repository cloned"

# 3. Build
echo "[3/6] Building OpenSyria (this takes 5-15 minutes)..."
cmake -B build -DBUILD_DAEMON=ON -DBUILD_CLI=ON -DBUILD_TESTS=OFF -DBUILD_GUI=OFF > /dev/null 2>&1
cmake --build build -j$(nproc) > /dev/null 2>&1
echo "✓ Build complete"

# 4. Configure
echo "[4/6] Configuring..."
mkdir -p ~/.opensyria
cat > ~/.opensyria/opensyria.conf << EOF
# OpenSyria Mining Configuration
server=1
daemon=0
printtoconsole=1
txindex=1

# Network
addnode=$SEED_NODE
addnode=$DNS_SEED
maxconnections=32

# RPC
rpcuser=miner
rpcpassword=minerpass$(date +%s | sha256sum | head -c 16)
rpcallowip=127.0.0.1
rpcbind=127.0.0.1
EOF
echo "✓ Configuration created"

# 5. Start daemon
echo "[5/6] Starting daemon..."
pkill opensyriad 2>/dev/null || true
sleep 2
screen -dmS opensyriad ./build/bin/opensyriad -printtoconsole

# Wait for daemon to start
echo "   Waiting for daemon to initialize..."
sleep 10

# Wait for sync
echo "   Syncing blockchain..."
for i in {1..60}; do
    BLOCKS=$(./build/bin/opensyria-cli getblockcount 2>/dev/null || echo "0")
    if [ "$BLOCKS" != "0" ] && [ "$BLOCKS" != "" ]; then
        echo "   Current block: $BLOCKS"
        break
    fi
    sleep 5
done
echo "✓ Daemon running and synced"

# 6. Start mining
echo "[6/6] Starting mining..."
screen -dmS miner bash -c "
while true; do
    /root/OpenSyria/build/bin/opensyria-cli generatetoaddress 1 $MINING_ADDRESS 500000000 2>/dev/null || sleep 5
done
"
echo "✓ Mining started!"

echo ""
echo "=========================================="
echo "  SETUP COMPLETE!"
echo "=========================================="
echo ""
echo "Commands:"
echo "  screen -r miner      # View mining output"
echo "  screen -r opensyriad # View daemon logs"
echo "  Ctrl+A, D            # Detach from screen"
echo ""
echo "Check status:"
echo "  ./build/bin/opensyria-cli getmininginfo"
echo "  ./build/bin/opensyria-cli getblockcount"
echo ""
echo "Mining to: $MINING_ADDRESS"
echo ""

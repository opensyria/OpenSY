# 🖥️ OpenSY Node Operator Guide

A complete guide to running an OpenSY full node.

---

## Quick Start

```bash
# Clone and build
git clone https://github.com/opensyria/OpenSY.git
cd OpenSY
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)

# Start node
./build/bin/opensyd -daemon -addnode=node1.opensy.net

# Check status
./build/bin/opensy-cli getblockchaininfo
```

---

## System Requirements

### Minimum
| Resource | Requirement |
|----------|-------------|
| CPU | 2 cores |
| RAM | 2 GB |
| Storage | 20 GB SSD |
| Network | 10 Mbps |
| OS | Ubuntu 22.04+, macOS 12+, Windows 10+ |

### Recommended
| Resource | Requirement |
|----------|-------------|
| CPU | 4+ cores |
| RAM | 4+ GB |
| Storage | 50+ GB SSD |
| Network | 100 Mbps |

---

## Installation

### Ubuntu/Debian

```bash
# Install dependencies
sudo apt update
sudo apt install -y build-essential cmake pkg-config \
  libboost-dev libevent-dev libsqlite3-dev libssl-dev

# Clone and build
git clone https://github.com/opensyria/OpenSY.git
cd OpenSY
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)

# Install (optional)
sudo cmake --install build
```

### macOS

```bash
# Install dependencies
brew install cmake boost libevent sqlite openssl

# Clone and build
git clone https://github.com/opensyria/OpenSY.git
cd OpenSY
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(sysctl -n hw.ncpu)
```

---

## Configuration

### Config File Location
| OS | Path |
|----|------|
| Linux | `~/.opensy/opensy.conf` |
| macOS | `~/Library/Application Support/OpenSY/opensy.conf` |
| Windows | `%APPDATA%\OpenSY\opensy.conf` |

### Basic Configuration

```ini
# ~/.opensy/opensy.conf

# Network
server=1
listen=1
port=9633

# Connections
maxconnections=125
addnode=node1.opensy.net

# RPC (for local access only)
rpcuser=opensy
rpcpassword=YOUR_SECURE_PASSWORD
rpcbind=127.0.0.1
rpcport=9632

# Logging
debug=net
logips=1
logtimestamps=1

# Performance
dbcache=450
maxmempool=300
```

### Seed Node Configuration

If running a public seed node:

```ini
# Additional settings for seed nodes
listen=1
discover=1
dns=1
dnsseed=1

# Allow more connections
maxconnections=256
```

---

## Running as a Service (Linux)

### Create systemd service

```bash
sudo tee /etc/systemd/system/opensyd.service << 'EOF'
[Unit]
Description=OpenSY Daemon
Documentation=https://opensy.net
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/opensyd -printtoconsole
ExecStop=/usr/local/bin/opensy-cli stop
Restart=on-failure
RestartSec=30
TimeoutStopSec=60

[Install]
WantedBy=multi-user.target
EOF
```

### Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable opensyd
sudo systemctl start opensyd

# Check status
sudo systemctl status opensyd

# View logs
sudo journalctl -u opensyd -f
```

---

## Firewall Configuration

### Required Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 9633 | TCP | P2P Network |
| 9632 | TCP | RPC (local only) |
| 53 | UDP | DNS Seeder (if running) |

### UFW (Ubuntu)

```bash
sudo ufw allow 9633/tcp comment "OpenSY P2P"
sudo ufw enable
sudo ufw status
```

### iptables

```bash
sudo iptables -A INPUT -p tcp --dport 9633 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 22 -j ACCEPT
sudo netfilter-persistent save
```

---

## Useful Commands

### Node Status

```bash
# Blockchain info
opensy-cli getblockchaininfo

# Network info
opensy-cli getnetworkinfo

# Connection count
opensy-cli getconnectioncount

# Peer list
opensy-cli getpeerinfo

# Memory pool
opensy-cli getmempoolinfo
```

### Wallet Operations

```bash
# List wallets
opensy-cli listwallets

# Create wallet
opensy-cli createwallet "my-wallet"

# Get balance
opensy-cli -rpcwallet=my-wallet getbalance

# Get new address
opensy-cli -rpcwallet=my-wallet getnewaddress
```

### Mining

```bash
# Get mining info
opensy-cli getmininginfo

# Mine blocks
opensy-cli generatetoaddress 10 YOUR_ADDRESS 500000000
```

### Maintenance

```bash
# Stop node gracefully
opensy-cli stop

# Get blockchain size
du -sh ~/.opensy/blocks/

# Verify blockchain
opensy-cli verifychain
```

---

## Monitoring

### Check sync progress

```bash
opensy-cli getblockchaininfo | grep -E "blocks|headers|verificationprogress"
```

### Monitor connections

```bash
watch -n 5 'opensy-cli getconnectioncount'
```

### Log monitoring

```bash
tail -f ~/.opensy/debug.log
```

---

## Troubleshooting

### Node won't start

```bash
# Check if already running
pgrep opensyd

# Check logs
tail -100 ~/.opensy/debug.log

# Try starting in foreground
opensyd -printtoconsole
```

### No connections

```bash
# Manually add a peer
opensy-cli addnode "node1.opensy.net" "add"

# Check if port is open
nc -zv node1.opensy.net 9633
```

### Sync stuck

```bash
# Check peer info
opensy-cli getpeerinfo | grep synced

# Restart node
opensy-cli stop && sleep 5 && opensyd -daemon
```

### Out of memory

Edit config to reduce memory usage:
```ini
dbcache=100
maxmempool=50
```

---

## Testnet

Run a testnet node for development and testing without risking real SYL.

### Starting Testnet

```bash
# Start testnet daemon
opensyd -testnet -daemon

# Check testnet status
opensy-cli -testnet getblockchaininfo

# Get testnet wallet address
opensy-cli -testnet getnewaddress
```

### Testnet Configuration

Add to your `opensy.conf`:

```ini
# Testnet section
[test]
server=1
listen=1
port=19633
rpcport=19632
rpcuser=opensy
rpcpassword=YOUR_TESTNET_PASSWORD
rpcbind=127.0.0.1
rpcallowip=127.0.0.1
addnode=node1.opensy.net:9633
```

### Testnet Ports

| Port | Purpose |
|------|---------|
| 19633 | Testnet P2P |
| 19632 | Testnet RPC |

### Running Testnet as Service

```bash
sudo tee /etc/systemd/system/opensyd-testnet.service << 'EOF'
[Unit]
Description=OpenSY Testnet Daemon
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/opensyd -testnet -printtoconsole
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable opensyd-testnet
sudo systemctl start opensyd-testnet
```

---

## Backups & Maintenance

### Automated Backup Script

Create `/opt/opensy/backup.sh`:

```bash
#!/bin/bash
BACKUP_DIR=/opt/opensy/backups
DATE=$(date +%Y%m%d_%H%M%S)
RETAIN_DAYS=7

mkdir -p $BACKUP_DIR

# Backup wallet
[ -f ~/.opensy/wallet.dat ] && cp ~/.opensy/wallet.dat $BACKUP_DIR/wallet_$DATE.dat

# Backup config
cp ~/.opensy/opensy.conf $BACKUP_DIR/opensy.conf_$DATE

# Backup peers database
[ -f ~/.opensy/peers.dat ] && cp ~/.opensy/peers.dat $BACKUP_DIR/peers_$DATE.dat

# Clean old backups
find $BACKUP_DIR -type f -mtime +$RETAIN_DAYS -delete

echo "[$(date)] Backup completed"
```

Schedule daily backup:
```bash
chmod +x /opt/opensy/backup.sh
(crontab -l; echo "0 2 * * * /opt/opensy/backup.sh >> /opt/opensy/backups/backup.log 2>&1") | crontab -
```

### Log Rotation

Create `/etc/logrotate.d/opensy`:

```
/home/ubuntu/.opensy/debug.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

### Swap Space (for low-memory servers)

```bash
# Create 2GB swap
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# Make permanent
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab

# Optimize swappiness
echo 'vm.swappiness=10' | sudo tee -a /etc/sysctl.conf
sudo sysctl vm.swappiness=10
```

---

## Security Best Practices

1. **Keep RPC local** - Never expose RPC to internet
2. **Use strong RPC password** - Generate with `openssl rand -hex 32`
3. **Firewall** - Only open port 9633 (and 19633 for testnet)
4. **Updates** - Keep software updated
5. **Backups** - Automated daily wallet backups
6. **Monitoring** - Set up alerts for downtime
7. **Swap space** - Add swap on low-memory VPS
8. **Log rotation** - Prevent disk fill from logs

---

## Network Information

| Parameter | Value |
|-----------|-------|
| **Mainnet P2P Port** | 9633 |
| **Mainnet RPC Port** | 9632 |
| **Testnet P2P Port** | 19633 |
| **Magic Bytes** | 0x53594c4d (SYLM) |
| **Address Prefix** | F (35) |
| **Bech32 Prefix** | syl |
| **DNS Seed** | seed.opensy.net |

---

## Getting Help

- **GitHub Issues:** https://github.com/opensyria/OpenSY/issues
- **Documentation:** https://github.com/opensyria/OpenSY/tree/main/docs

---

**سوريا حرة** 🇸🇾

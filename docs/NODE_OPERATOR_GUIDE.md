# 🖥️ OpenSyria Node Operator Guide

A complete guide to running an OpenSyria full node.

---

## Quick Start

```bash
# Clone and build
git clone https://github.com/opensyria/OpenSyria.git
cd OpenSyria
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)

# Start node
./build/bin/opensyriad -daemon -addnode=node1.opensyria.net

# Check status
./build/bin/opensyria-cli getblockchaininfo
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
git clone https://github.com/opensyria/OpenSyria.git
cd OpenSyria
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
git clone https://github.com/opensyria/OpenSyria.git
cd OpenSyria
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(sysctl -n hw.ncpu)
```

---

## Configuration

### Config File Location
| OS | Path |
|----|------|
| Linux | `~/.opensyria/opensyria.conf` |
| macOS | `~/Library/Application Support/OpenSyria/opensyria.conf` |
| Windows | `%APPDATA%\OpenSyria\opensyria.conf` |

### Basic Configuration

```ini
# ~/.opensyria/opensyria.conf

# Network
server=1
listen=1
port=9633

# Connections
maxconnections=125
addnode=node1.opensyria.net

# RPC (for local access only)
rpcuser=opensyria
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
sudo tee /etc/systemd/system/opensyriad.service << 'EOF'
[Unit]
Description=OpenSyria Core Daemon
Documentation=https://opensyria.net
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/opensyriad -printtoconsole
ExecStop=/usr/local/bin/opensyria-cli stop
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
sudo systemctl enable opensyriad
sudo systemctl start opensyriad

# Check status
sudo systemctl status opensyriad

# View logs
sudo journalctl -u opensyriad -f
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
sudo ufw allow 9633/tcp comment "OpenSyria P2P"
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
opensyria-cli getblockchaininfo

# Network info
opensyria-cli getnetworkinfo

# Connection count
opensyria-cli getconnectioncount

# Peer list
opensyria-cli getpeerinfo

# Memory pool
opensyria-cli getmempoolinfo
```

### Wallet Operations

```bash
# List wallets
opensyria-cli listwallets

# Create wallet
opensyria-cli createwallet "my-wallet"

# Get balance
opensyria-cli -rpcwallet=my-wallet getbalance

# Get new address
opensyria-cli -rpcwallet=my-wallet getnewaddress
```

### Mining

```bash
# Get mining info
opensyria-cli getmininginfo

# Mine blocks
opensyria-cli generatetoaddress 10 YOUR_ADDRESS 500000000
```

### Maintenance

```bash
# Stop node gracefully
opensyria-cli stop

# Get blockchain size
du -sh ~/.opensyria/blocks/

# Verify blockchain
opensyria-cli verifychain
```

---

## Monitoring

### Check sync progress

```bash
opensyria-cli getblockchaininfo | grep -E "blocks|headers|verificationprogress"
```

### Monitor connections

```bash
watch -n 5 'opensyria-cli getconnectioncount'
```

### Log monitoring

```bash
tail -f ~/.opensyria/debug.log
```

---

## Troubleshooting

### Node won't start

```bash
# Check if already running
pgrep opensyriad

# Check logs
tail -100 ~/.opensyria/debug.log

# Try starting in foreground
opensyriad -printtoconsole
```

### No connections

```bash
# Manually add a peer
opensyria-cli addnode "node1.opensyria.net" "add"

# Check if port is open
nc -zv node1.opensyria.net 9633
```

### Sync stuck

```bash
# Check peer info
opensyria-cli getpeerinfo | grep synced

# Restart node
opensyria-cli stop && sleep 5 && opensyriad -daemon
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
opensyriad -testnet -daemon

# Check testnet status
opensyria-cli -testnet getblockchaininfo

# Get testnet wallet address
opensyria-cli -testnet getnewaddress
```

### Testnet Configuration

Add to your `opensyria.conf`:

```ini
# Testnet section
[test]
server=1
listen=1
port=19633
rpcport=19632
rpcuser=opensyria
rpcpassword=YOUR_TESTNET_PASSWORD
rpcbind=127.0.0.1
rpcallowip=127.0.0.1
addnode=node1.opensyria.net:9633
```

### Testnet Ports

| Port | Purpose |
|------|---------|
| 19633 | Testnet P2P |
| 19632 | Testnet RPC |

### Running Testnet as Service

```bash
sudo tee /etc/systemd/system/opensyriad-testnet.service << 'EOF'
[Unit]
Description=OpenSyria Testnet Daemon
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/opensyriad -testnet -printtoconsole
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable opensyriad-testnet
sudo systemctl start opensyriad-testnet
```

---

## Backups & Maintenance

### Automated Backup Script

Create `/opt/opensyria/backup.sh`:

```bash
#!/bin/bash
BACKUP_DIR=/opt/opensyria/backups
DATE=$(date +%Y%m%d_%H%M%S)
RETAIN_DAYS=7

mkdir -p $BACKUP_DIR

# Backup wallet
[ -f ~/.opensyria/wallet.dat ] && cp ~/.opensyria/wallet.dat $BACKUP_DIR/wallet_$DATE.dat

# Backup config
cp ~/.opensyria/opensyria.conf $BACKUP_DIR/opensyria.conf_$DATE

# Backup peers database
[ -f ~/.opensyria/peers.dat ] && cp ~/.opensyria/peers.dat $BACKUP_DIR/peers_$DATE.dat

# Clean old backups
find $BACKUP_DIR -type f -mtime +$RETAIN_DAYS -delete

echo "[$(date)] Backup completed"
```

Schedule daily backup:
```bash
chmod +x /opt/opensyria/backup.sh
(crontab -l; echo "0 2 * * * /opt/opensyria/backup.sh >> /opt/opensyria/backups/backup.log 2>&1") | crontab -
```

### Log Rotation

Create `/etc/logrotate.d/opensyria`:

```
/home/ubuntu/.opensyria/debug.log {
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
| **DNS Seed** | seed.opensyria.net |

---

## Getting Help

- **GitHub Issues:** https://github.com/opensyria/OpenSyria/issues
- **Documentation:** https://github.com/opensyria/OpenSyria/tree/main/docs

---

**سوريا حرة** 🇸🇾

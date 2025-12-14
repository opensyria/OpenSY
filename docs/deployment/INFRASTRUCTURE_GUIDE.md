# OpenSY Infrastructure Deployment Guide

## AWS + Cloudflare Production Deployment

**Total Monthly Cost: ~$15-50/month (scales with usage)**

This guide walks you through deploying production-ready OpenSY blockchain infrastructure using AWS and Cloudflare.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Prerequisites](#2-prerequisites)
3. [Phase 1: AWS Account Setup](#3-phase-1-aws-account-setup)
4. [Phase 2: Launch First Seed Node](#4-phase-2-launch-first-seed-node)
5. [Phase 3: Server Configuration](#5-phase-3-server-configuration)
6. [Phase 4: Build & Install OpenSY](#6-phase-4-build--install-opensy)
7. [Phase 5: Configure DNS (Cloudflare)](#7-phase-5-configure-dns-cloudflare)
8. [Phase 6: DNS Seeder Setup](#8-phase-6-dns-seeder-setup)
9. [Phase 7: Block Explorer](#9-phase-7-block-explorer)
10. [Phase 8: Monitoring](#10-phase-8-monitoring)
11. [Scaling & Multi-Region](#11-scaling--multi-region)
12. [Maintenance](#12-maintenance)

---

## 1. Architecture Overview

```
                              INTERNET
                                  │
                                  ▼
              ┌───────────────────────────────────────┐
              │         CLOUDFLARE (FREE)             │
              │  • DNS Management                     │
              │  • DDoS Protection                    │
              │  • SSL/TLS                            │
              └───────────────────────────────────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
┌───────────────┐       ┌───────────────┐       ┌───────────────┐
│  AWS EC2      │       │  AWS EC2      │       │  AWS EC2      │
│  SEED NODE 1  │       │  SEED NODE 2  │       │  EXPLORER     │
│  (Primary)    │       │  (Secondary)  │       │  (Optional)   │
│               │       │               │       │               │
│  t3.small     │       │  t3.micro     │       │  t3.small     │
│  $15/mo       │       │  Free tier    │       │  $15/mo       │
│               │       │  or $8/mo     │       │               │
│  + DNS Seeder │       │               │       │  + Nginx      │
└───────────────┘       └───────────────┘       └───────────────┘

Domain: opensy.net (Namecheap → Cloudflare DNS)

Cost Breakdown (Minimum):
├── AWS EC2 t3.small (seed1):     ~$15/month
├── AWS EC2 t2.micro (seed2):     $0 (Free tier) or ~$8/month
├── AWS EBS Storage (30GB each):  ~$3/month
├── AWS Data Transfer:            ~$5/month (first 100GB free)
├── Cloudflare:                   $0 (Free plan)
├── Domain:                       ~$1/month ($12/year)
└── TOTAL:                        ~$15-30/month
```

---

## 2. Prerequisites

### 2.1 Accounts Required

| Service | URL | Status |
|---------|-----|--------|
| ✅ AWS | https://aws.amazon.com | Your account ready |
| ✅ Cloudflare | https://cloudflare.com | Active, DNS configured |
| ✅ Namecheap | https://namecheap.com | Domain: opensy.net |

### 2.2 Local Requirements

```bash
# SSH key for server access
ssh-keygen -t ed25519 -C "opensy-aws"
cat ~/.ssh/id_ed25519.pub  # Copy this for AWS
```

### 2.3 Current Status (Updated Dec 8, 2025)

- [x] Domain `opensy.net` registered
- [x] Cloudflare DNS active
- [x] Nameservers configured
- [x] AWS account created
- [x] EC2 instance launched (157.175.40.131, Bahrain me-south-1)
- [x] OpenSY v30.99.0 built and running
- [x] Genesis chain mined (7,000+ blocks)
- [x] DNS seeder operational (seed.opensy.net)
- [x] External peers connecting! ✅ **NETWORK IS LIVE**

---

## 3. Phase 1: AWS Account Setup

### 3.1 Enable Free Tier Alerts

1. Go to **AWS Console** → **Billing** → **Budgets**
2. Create budget:
   - Budget type: **Cost budget**
   - Budget amount: **$10** (alerts before charges)
   - Alert threshold: **80%**
   - Email: your email

### 3.2 Create IAM User (Best Practice)

1. Go to **IAM** → **Users** → **Add user**
2. Username: `opensy-admin`
3. Access: **AWS Management Console**
4. Permissions: **AdministratorAccess** (or EC2FullAccess for limited)
5. Save credentials securely

### 3.3 Choose Your Region

**Recommended regions for Middle East:**
| Region | Code | Latency |
|--------|------|---------|
| Bahrain | me-south-1 | Best for ME |
| Frankfurt | eu-central-1 | Good for EU/ME |
| Mumbai | ap-south-1 | Good for Asia |
| N. Virginia | us-east-1 | Most services |

> **Tip:** Start with **me-south-1** (Bahrain) for lowest latency to Syria/Middle East.

---

## 4. Phase 2: Launch First Seed Node

### 4.1 Go to EC2 Dashboard

1. AWS Console → **EC2** → **Launch Instance**

### 4.2 Configure Instance

#### Basic Settings
| Setting | Value |
|---------|-------|
| **Name** | `opensy-seed-1` |
| **AMI** | Ubuntu Server 24.04 LTS (64-bit x86) |
| **Architecture** | 64-bit (x86) |

#### Instance Type
| Option | Specs | Cost | Recommendation |
|--------|-------|------|----------------|
| `t2.micro` | 1 vCPU, 1GB RAM | **Free tier** | Testing only |
| `t3.micro` | 2 vCPU, 1GB RAM | ~$8/mo | Minimum for production |
| `t3.small` | 2 vCPU, 2GB RAM | ~$15/mo | **Recommended** |
| `t3.medium` | 2 vCPU, 4GB RAM | ~$30/mo | Heavy usage |

> **Choose `t3.small`** for reliable seed node operation.

#### Key Pair
1. Click **Create new key pair**
2. Name: `opensy-key`
3. Type: **ED25519** (or RSA)
4. Format: **.pem**
5. **Download and save securely!**

```bash
# After download, secure the key
mv ~/Downloads/opensy-key.pem ~/.ssh/
chmod 400 ~/.ssh/opensy-key.pem
```

#### Network Settings (Security Group)

Click **Edit** and configure:

| Type | Protocol | Port | Source | Description |
|------|----------|------|--------|-------------|
| SSH | TCP | 22 | My IP | SSH access |
| Custom TCP | TCP | **9633** | 0.0.0.0/0 | OpenSY P2P |
| Custom TCP | TCP | **9632** | My IP | RPC (restricted) |
| Custom TCP | TCP | **19633** | 0.0.0.0/0 | Testnet P2P |
| Custom UDP | UDP | **53** | 0.0.0.0/0 | DNS Seeder |
| Custom TCP | TCP | **53** | 0.0.0.0/0 | DNS Seeder |

> **Security Group Name:** `opensy-sg`

#### Storage

| Setting | Value |
|---------|-------|
| Size | **30 GiB** |
| Type | **gp3** |
| IOPS | 3000 (default) |
| Throughput | 125 (default) |

> 30GB is sufficient for initial blockchain. Can expand later.

### 4.3 Launch!

1. Review all settings
2. Click **Launch Instance**
3. Wait for status: **Running**
4. Note the **Public IPv4 address**

### 4.4 Allocate Elastic IP (Recommended)

Static IP that persists across restarts:

1. EC2 → **Elastic IPs** → **Allocate**
2. Click **Allocate**
3. Select the new IP → **Actions** → **Associate**
4. Choose your instance
5. **Associate**

> Now your server has a permanent IP address.

---

## 5. Phase 3: Server Configuration

### 5.1 Connect via SSH

```bash
# Connect to your server
ssh -i ~/.ssh/opensy-key.pem ubuntu@YOUR_PUBLIC_IP

# Example:
ssh -i ~/.ssh/opensy-key.pem ubuntu@3.28.123.45
```

### 5.2 Initial System Setup

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Set timezone
sudo timedatectl set-timezone Asia/Riyadh  # Or your preference

# Set hostname
sudo hostnamectl set-hostname opensy-seed-1

# Install essential packages
sudo apt install -y \
    build-essential \
    cmake \
    pkg-config \
    git \
    ufw \
    fail2ban \
    htop \
    tmux \
    curl \
    wget \
    unzip
```

### 5.3 Configure Firewall (UFW)

```bash
# Reset and configure UFW
sudo ufw --force reset
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow required ports
sudo ufw allow 22/tcp comment 'SSH'
sudo ufw allow 9633/tcp comment 'OpenSY P2P'
sudo ufw allow 9632/tcp comment 'OpenSY RPC'
sudo ufw allow 19633/tcp comment 'OpenSY Testnet'
sudo ufw allow 53/udp comment 'DNS Seeder'
sudo ufw allow 53/tcp comment 'DNS Seeder'

# Enable firewall
sudo ufw --force enable
sudo ufw status verbose
```

### 5.4 Configure Fail2Ban

```bash
# Enable and start fail2ban
sudo systemctl enable fail2ban
sudo systemctl start fail2ban

# Check status
sudo fail2ban-client status
```

### 5.5 Create OpenSY User

```bash
# Create dedicated user
sudo useradd -m -s /bin/bash opensy
sudo usermod -aG sudo opensy

# Create directories
sudo mkdir -p /opt/opensy
sudo chown opensy:opensy /opt/opensy
```

---

## 6. Phase 4: Build & Install OpenSY

### 6.1 Install Build Dependencies

```bash
# Install all dependencies
sudo apt install -y \
    libboost-all-dev \
    libevent-dev \
    libsqlite3-dev \
    libzmq3-dev \
    libssl-dev \
    libminiupnpc-dev \
    libnatpmp-dev \
    systemtap-sdt-dev
```

### 6.2 Clone and Build

```bash
# Switch to opensy user
sudo su - opensy

# Clone repository
cd /opt/opensy
git clone https://github.com/opensyria/OpenSY.git source
cd source

# Build (daemon and CLI only, no GUI)
cmake -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_DAEMON=ON \
    -DBUILD_CLI=ON \
    -DBUILD_GUI=OFF \
    -DBUILD_TESTS=OFF

# Compile (takes 10-30 minutes)
cmake --build build -j$(nproc)

# Verify build
./build/bin/opensyd --version
./build/bin/opensy-cli --version
```

### 6.3 Install Binaries

```bash
# Install system-wide
sudo cmake --install build --prefix /usr/local

# Verify installation
which opensyd
opensyd --version
```

### 6.4 Configure OpenSY Node

```bash
# Create config directory (as opensy user)
mkdir -p ~/.opensy

# Generate secure RPC password
RPC_PASS=$(openssl rand -hex 32)

# Create configuration
cat > ~/.opensy/opensy.conf << EOF
# OpenSY Seed Node Configuration
# Server: seed1.opensy.net

# =============
# NETWORK
# =============
server=1
daemon=1
listen=1
port=9633
bind=0.0.0.0

# Peer connections
maxconnections=256
maxuploadtarget=5000

# =============
# RPC
# =============
rpcuser=opensyrpc
rpcpassword=${RPC_PASS}
rpcbind=127.0.0.1
rpcallowip=127.0.0.1
rpcport=9632

# =============
# INDEXING
# =============
txindex=1

# =============
# LOGGING
# =============
debug=net
logips=1
logtimestamps=1
shrinkdebugfile=1

# =============
# PERFORMANCE
# =============
dbcache=512
par=2

# =============
# SEEDS
# =============
# Will be populated as more nodes come online
# seednode=seed2.opensy.net:9633
EOF

# Save credentials for reference
echo "RPC Password: ${RPC_PASS}" > ~/.opensy/.rpc_credentials
chmod 600 ~/.opensy/.rpc_credentials

# Display password (save this!)
echo "=========================================="
echo "SAVE THIS RPC PASSWORD:"
echo "${RPC_PASS}"
echo "=========================================="
```

### 6.5 Create Systemd Service

```bash
# Exit to ubuntu user for sudo
exit

# Create service file
sudo tee /etc/systemd/system/opensyd.service << 'EOF'
[Unit]
Description=OpenSY Daemon
Documentation=https://opensy.net/
After=network-online.target
Wants=network-online.target

[Service]
Type=forking
User=opensy
Group=opensy

ExecStart=/usr/local/bin/opensyd -daemon -conf=/home/opensy/.opensy/opensy.conf -datadir=/home/opensy/.opensy
ExecStop=/usr/local/bin/opensy-cli -conf=/home/opensy/.opensy/opensy.conf stop

Restart=on-failure
RestartSec=30
TimeoutStartSec=infinity
TimeoutStopSec=600

# Security hardening
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true
PrivateDevices=true

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable opensyd
sudo systemctl start opensyd

# Check status
sudo systemctl status opensyd
```

### 6.6 Verify Node Operation

```bash
# Switch to opensy user
sudo su - opensy

# Check blockchain info
opensy-cli getblockchaininfo

# Check network info
opensy-cli getnetworkinfo

# Check peer connections (will be empty initially)
opensy-cli getpeerinfo

# Check if listening
opensy-cli getnetworkinfo | grep -A5 "localaddresses"
```

---

## 7. Phase 5: Configure DNS (Cloudflare)

### 7.1 Get Your Server IP

```bash
# On server
curl ifconfig.me
```

### 7.2 Add DNS Records in Cloudflare

Go to Cloudflare Dashboard → `opensy.net` → **DNS**

Add these records:

| Type | Name | Content | Proxy | TTL |
|------|------|---------|-------|-----|
| A | `@` | YOUR_SERVER_IP | **Proxied** (orange) | Auto |
| A | `www` | YOUR_SERVER_IP | **Proxied** (orange) | Auto |
| A | `node1` | YOUR_SERVER_IP | **DNS only** (grey) | Auto |
| A | `ns1` | YOUR_SERVER_IP | **DNS only** (grey) | Auto |

> **Important:** Seed nodes must have Proxy **OFF** (grey cloud) for P2P to work!

### 7.3 Add NS Record for DNS Seeder

| Type | Name | Content | TTL |
|------|------|---------|-----|
| NS | `seed` | `ns1.opensy.net` | Auto |

This delegates `seed.opensy.net` to your DNS seeder.

### 7.4 Verify DNS

```bash
# Test from your local machine
dig node1.opensy.net
dig ns1.opensy.net

# Should return your server IP
```

---

## 8. Phase 6: DNS Seeder Setup

### 8.1 Copy Pre-built Seeder

The seeder is already in your repo at `/contrib/seeder/opensy-seeder/`

```bash
# On server, as opensy user
sudo su - opensy

cd /opt/opensy/source/contrib/seeder/opensy-seeder

# Build if not already built
make

# Verify
./dnsseed --help
```

### 8.2 Test DNS Seeder

```bash
# Test run (foreground)
./dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net -p 5353

# Press Ctrl+C to stop after testing
```

### 8.3 Create Seeder Service

```bash
# Exit to ubuntu user
exit

# Create service
sudo tee /etc/systemd/system/opensy-seeder.service << 'EOF'
[Unit]
Description=OpenSY DNS Seeder
After=network.target opensyd.service
Wants=opensyd.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/opensy/source/contrib/seeder/opensy-seeder
ExecStart=/opt/opensy/source/contrib/seeder/opensy-seeder/dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net
Restart=always
RestartSec=30

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable opensy-seeder
sudo systemctl start opensy-seeder

# Check status
sudo systemctl status opensy-seeder
```

> **Note:** DNS seeder runs as root to bind to port 53.

### 8.4 Verify DNS Seeder

```bash
# From your local machine
dig seed.opensy.net

# Should eventually return node IPs once peers connect
```

---

## 9. Phase 7: Block Explorer

### 9.1 Option A: Simple RPC Explorer (Lightweight)

```bash
# Install Node.js
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Install btc-rpc-explorer
sudo npm install -g btc-rpc-explorer

# Create config
mkdir -p ~/.config/btc-rpc-explorer
cat > ~/.config/btc-rpc-explorer/.env << EOF
BTCEXP_HOST=127.0.0.1
BTCEXP_PORT=3002
BTCEXP_BITCOIND_HOST=127.0.0.1
BTCEXP_BITCOIND_PORT=9632
BTCEXP_BITCOIND_USER=opensyrpc
BTCEXP_BITCOIND_PASS=YOUR_RPC_PASSWORD
BTCEXP_SITE_TITLE=OpenSY Explorer
BTCEXP_NO_RATES=true
EOF

# Create service
sudo tee /etc/systemd/system/opensy-explorer.service << 'EOF'
[Unit]
Description=OpenSY Block Explorer
After=opensyd.service

[Service]
Type=simple
User=opensy
ExecStart=/usr/bin/btc-rpc-explorer
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable opensy-explorer
sudo systemctl start opensy-explorer
```

### 9.2 Setup Nginx Reverse Proxy

```bash
# Install Nginx
sudo apt install -y nginx certbot python3-certbot-nginx

# Create config
sudo tee /etc/nginx/sites-available/explorer << 'EOF'
server {
    listen 80;
    server_name explorer.opensy.net opensy.net www.opensy.net;

    location / {
        proxy_pass http://127.0.0.1:3002;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

# Enable site
sudo ln -sf /etc/nginx/sites-available/explorer /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

### 9.3 Add Explorer DNS Record

In Cloudflare, add:

| Type | Name | Content | Proxy |
|------|------|---------|-------|
| A | `explorer` | YOUR_SERVER_IP | **Proxied** (orange) |

### 9.4 Enable HTTPS (After DNS propagates)

```bash
# Get SSL certificate
sudo certbot --nginx -d opensy.net -d www.opensy.net -d explorer.opensy.net

# Auto-renewal is configured automatically
```

---

## 10. Phase 8: Monitoring

### 10.1 Basic Monitoring Script

```bash
# Create monitoring script
sudo tee /opt/opensy/monitor.sh << 'EOF'
#!/bin/bash
echo "=== OpenSY Node Status ==="
echo "Date: $(date)"
echo ""

echo "=== Blockchain Info ==="
opensy-cli getblockchaininfo | grep -E "chain|blocks|headers|verificationprogress"
echo ""

echo "=== Network Info ==="
opensy-cli getnetworkinfo | grep -E "connections|version"
echo ""

echo "=== System Resources ==="
echo "Disk: $(df -h / | tail -1 | awk '{print $5 " used"}')"
echo "Memory: $(free -h | grep Mem | awk '{print $3 "/" $2}')"
echo "Load: $(uptime | awk -F'load average:' '{print $2}')"
echo ""

echo "=== Services ==="
systemctl is-active opensyd && echo "opensyd: running" || echo "opensyd: STOPPED"
systemctl is-active opensy-seeder && echo "seeder: running" || echo "seeder: STOPPED"
EOF

chmod +x /opt/opensy/monitor.sh

# Run it
/opt/opensy/monitor.sh
```

### 10.2 CloudWatch Monitoring (AWS)

1. EC2 → Select instance → **Monitoring** tab
2. Click **Manage detailed monitoring** → Enable
3. Create alarms for:
   - CPU > 80%
   - Disk > 80%
   - Status check failed

---

## 11. Scaling & Multi-Region

### 11.1 Add Second Seed Node

1. Launch new EC2 instance in different region (e.g., `eu-central-1`)
2. Repeat Phase 3-6
3. Add DNS records:
   - `node2.opensy.net` → new IP
   - `seed2.opensy.net` → new IP

### 11.2 Update chainparams.cpp

After multiple nodes are running:

```cpp
// In src/kernel/chainparams.cpp
vSeeds.emplace_back("seed.opensy.net");
vSeeds.emplace_back("seed2.opensy.net");

// Fixed seeds (backup)
// Run: contrib/seeds/generate-seeds.py
```

### 11.3 AWS Multi-Region Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   me-south-1    │    │  eu-central-1   │    │  ap-south-1     │
│   (Bahrain)     │    │  (Frankfurt)    │    │  (Mumbai)       │
│                 │    │                 │    │                 │
│  seed1 + seeder │    │     seed2       │    │     seed3       │
│     $15/mo      │    │     $15/mo      │    │     $15/mo      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                     │                     │
         └─────────────────────┼─────────────────────┘
                               │
                    ┌──────────┴──────────┐
                    │  Route 53 / Cloudflare │
                    │  Latency-based DNS     │
                    └─────────────────────────┘
```

---

## 12. Maintenance

### 12.1 Regular Tasks

```bash
# Daily: Check node status
/opt/opensy/monitor.sh

# Weekly: Update system
sudo apt update && sudo apt upgrade -y

# Weekly: Check logs
journalctl -u opensyd --since "1 week ago" | tail -100

# Monthly: Rotate RPC password
NEW_PASS=$(openssl rand -hex 32)
sed -i "s/rpcpassword=.*/rpcpassword=${NEW_PASS}/" ~/.opensy/opensy.conf
sudo systemctl restart opensyd
```

### 12.2 Update OpenSY

```bash
cd /opt/opensy/source
git pull origin main
cmake --build build -j$(nproc)
sudo cmake --install build --prefix /usr/local
sudo systemctl restart opensyd
```

### 12.3 Backup

```bash
# Backup wallet (if any)
opensy-cli backupwallet ~/wallet-backup-$(date +%Y%m%d).dat

# Copy to S3
aws s3 cp ~/wallet-backup-*.dat s3://your-bucket/backups/
```

---

## Quick Reference

### Service Commands

```bash
# Node
sudo systemctl start|stop|restart|status opensyd

# Seeder
sudo systemctl start|stop|restart|status opensy-seeder

# Explorer
sudo systemctl start|stop|restart|status opensy-explorer
```

### Useful CLI Commands

```bash
# Blockchain
opensy-cli getblockchaininfo
opensy-cli getblockcount
opensy-cli getbestblockhash

# Network
opensy-cli getnetworkinfo
opensy-cli getpeerinfo
opensy-cli getconnectioncount

# Mining (testnet/regtest)
opensy-cli generatetoaddress 1 $(opensy-cli getnewaddress)

# Wallet
opensy-cli createwallet "main"
opensy-cli getnewaddress
opensy-cli getbalance
```

### Port Reference

| Port | Protocol | Service |
|------|----------|---------|
| 9633 | TCP | P2P Mainnet |
| 9632 | TCP | RPC |
| 19633 | TCP | P2P Testnet |
| 53 | UDP/TCP | DNS Seeder |
| 80/443 | TCP | Web/Explorer |

### URLs (After Deployment)

- **Website:** https://opensy.net
- **Explorer:** https://explorer.opensy.net
- **Seed DNS:** seed.opensy.net

---

**مشروع سوريا المفتوحة - Syria's First Blockchain** 🇸🇾

*Last Updated: December 2025*

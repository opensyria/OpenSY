# OpenSY Deployment Scripts

This directory contains scripts and configurations for deploying OpenSY infrastructure.

## Quick Start

### Option 1: One-Line Node Setup

```bash
# On a fresh Ubuntu 22.04 server:
curl -sSL https://raw.githubusercontent.com/opensy/OpenSY/main/contrib/deploy/setup-node.sh | sudo bash
```

### Option 2: Manual Setup

1. Download the scripts:
```bash
git clone https://github.com/opensyria/OpenSY.git
cd OpenSY/contrib/deploy
```

2. Run setup scripts:
```bash
# Basic node
sudo ./setup-node.sh

# DNS seeder (on seed node 1)
sudo ./setup-dns-seeder.sh

# Block explorer (on explorer server)
sudo ./setup-explorer.sh
```

## Directory Structure

```
contrib/deploy/
├── README.md                    # This file
├── setup-node.sh                # Full node setup script
├── setup-dns-seeder.sh          # DNS seeder setup script  
├── setup-explorer.sh            # Block explorer setup script
└── docker/
    ├── Dockerfile               # OpenSY node Docker image
    ├── docker-compose.yml       # Multi-service composition
    └── opensy.conf.default   # Default Docker config
```

## Deployment Guide

For complete deployment instructions, see:
- [Infrastructure Guide](../../docs/deployment/INFRASTRUCTURE_GUIDE.md)

## Recommended Infrastructure

### Free Tier (Bootstrapping)
- **Oracle Cloud Free**: 2x ARM nodes (4 OCPU, 24GB total)
- **Hetzner CX22**: €3.29/month per node
- **Cloudflare**: Free DNS & DDoS protection

### Production (Growth)
- **AWS/GCP**: Enterprise-grade infrastructure
- See the Infrastructure Guide for migration path

## Network Ports

| Port | Protocol | Service |
|------|----------|---------|
| 9633 | TCP | P2P Mainnet |
| 9632 | TCP | RPC |
| 19633 | TCP | P2P Testnet |
| 53 | UDP/TCP | DNS Seeder |
| 80 | TCP | HTTP |
| 443 | TCP | HTTPS |

## Docker Usage

### Build Image
```bash
cd /path/to/OpenSY
docker build -t opensy/node:latest -f contrib/deploy/docker/Dockerfile .
```

### Run Node
```bash
docker run -d \
  --name opensy-node \
  -p 9633:9633 \
  -v opensy-data:/home/opensy/.opensy \
  opensy/node:latest
```

### Docker Compose
```bash
cd contrib/deploy/docker

# Start node only
docker compose up -d opensyd

# Start node + explorer
docker compose --profile explorer up -d

# Start everything (node + explorer + monitoring)
docker compose --profile explorer --profile monitoring up -d
```

## Updating Nodes

### Manual Update
```bash
cd /opt/opensy/source
git pull origin main
cmake --build build -j$(nproc)
sudo systemctl restart opensyd
```

### Docker Update
```bash
cd contrib/deploy/docker
docker compose pull
docker compose up -d
```

## Troubleshooting

### Node won't start
```bash
# Check logs
journalctl -u opensyd -f

# Check configuration
cat /home/opensy/.opensy/opensy.conf
```

### Can't connect to peers
```bash
# Check firewall
sudo ufw status

# Check if port is open
nc -zv seed.opensy.net 9633

# Manual peer connection
opensy-cli addnode <IP>:9633 onetry
```

### RPC not responding
```bash
# Check if running
opensy-cli getblockchaininfo

# Check RPC config
grep rpc /home/opensy/.opensy/opensy.conf
```

## Security Considerations

1. **Never expose RPC to the internet** without authentication
2. **Use strong RPC passwords** (generated automatically by setup scripts)
3. **Keep systems updated**: `apt update && apt upgrade -y`
4. **Monitor logs** for suspicious activity
5. **Use fail2ban** to prevent brute force attacks

## Support

- **Documentation**: https://opensy.net/docs
- **GitHub Issues**: https://github.com/opensyria/OpenSY/issues
- **Email**: admin@opensy.net

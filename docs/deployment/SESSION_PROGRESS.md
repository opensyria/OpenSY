# OpenSY Deployment Progress

**Last Updated:** December 7, 2025  
**Status:** 🚀 AWS Ready - Deploying First Node  

---

## ✅ Completed

### 1. Blockchain Development (100%)
- [x] Bitcoin Core v30.99.0 fork complete
- [x] Currency: SYL (Syrian Lira Digital)
- [x] Genesis block mined (Dec 8, 2024 - Syria Liberated from Assad)
- [x] All network parameters configured
- [x] Builds successfully (Qt, CLI, daemon)

### 2. DNS Seeder (100%)
- [x] Cloned and rebranded from bitcoin-seeder
- [x] Location: `/contrib/seeder/opensy-seeder/`
- [x] Magic bytes: `0x53594c4d` (SYLM)
- [x] Port: 9633 (mainnet), 19633 (testnet)
- [x] Default seed: `seed.opensy.net`
- [x] Binary compiled: `dnsseed` (186KB arm64)

### 3. Deployment Infrastructure (100%)
- [x] Setup scripts: `/contrib/deploy/`
  - `setup-node.sh` - Full node setup
  - `setup-dns-seeder.sh` - DNS seeder setup
  - `setup-explorer.sh` - Block explorer setup
- [x] Docker configs: `/contrib/deploy/docker/`
- [x] Comprehensive guide: `/docs/deployment/INFRASTRUCTURE_GUIDE.md`

### 4. Domain & DNS (100%)
- [x] Domain acquired: **opensy.net**
- [x] Registrar: Namecheap
- [x] DNS: Cloudflare (Active)
- [x] Nameservers configured
- [x] Parking page records removed

### 5. Cloud Accounts
- [x] **AWS Account** ✅ (Active - using this!)
- [x] Oracle Cloud (Riyadh) - capacity issues, abandoned
- [x] Hetzner - ID verification failed, abandoned

---

## ⏳ In Progress

### AWS EC2 Deployment
**Status:** Ready to launch first instance

**Instance Configuration:**
- **Name:** `opensy-seed-1`
- **AMI:** Ubuntu Server 24.04 LTS
- **Type:** `t2.micro` (Free tier) or `t3.small` ($15/mo)
- **Storage:** 30 GB gp3
- **Security Group Ports:**
  - SSH (22) - Your IP only
  - OpenSY P2P (9633) - Anywhere
  - OpenSY RPC (9632) - Your IP only

---

## 📋 Next Steps (Priority Order)

### Step 1: Launch AWS EC2 Instance ⬅️ YOU ARE HERE
1. Go to **AWS Console** → EC2 → **Launch Instance**
2. Configure:
   - **Name:** `opensy-seed-1`
   - **AMI:** Ubuntu Server 24.04 LTS (Free tier eligible)
   - **Instance type:** `t2.micro` (Free tier) or `t3.small` (~$15/mo)
   - **Key pair:** Create new, download `.pem` file (SAVE IT!)
   - **Security Group:** Add ports 22, 9633, 9632
   - **Storage:** 30 GB gp3
3. Launch and note the **Public IP**

### Step 2: SSH & Setup Node
```bash
# Make key usable
chmod 400 ~/Downloads/opensy-seed-1.pem

# SSH into server
ssh -i ~/Downloads/opensy-seed-1.pem ubuntu@<PUBLIC-IP>

# Run setup (on server)
sudo apt update && sudo apt install -y git build-essential cmake pkg-config \
  libboost-all-dev libevent-dev libsqlite3-dev

git clone https://github.com/opensyria/OpenSY.git
cd OpenSY
cmake -B build -DBUILD_DAEMON=ON -DBUILD_CLI=ON
cmake --build build -j$(nproc)
sudo cmake --install build

# Create config
mkdir -p ~/.opensy
cat > ~/.opensy/opensy.conf << 'EOF'
server=1
daemon=1
listen=1
rpcuser=opensyrpc
rpcpassword=CHANGE_THIS_TO_RANDOM_STRING
rpcallowip=127.0.0.1
EOF

# Start daemon
opensyd -daemon
opensy-cli getblockchaininfo
```

### Step 3: Configure Cloudflare DNS
After node is running, add these records in Cloudflare:
| Type | Name | Content | Proxy |
|------|------|---------|-------|
| A | node1 | `<EC2-PUBLIC-IP>` | DNS only (grey) |
| A | @ | `<EC2-PUBLIC-IP>` | Proxied (orange) |
| A | www | `<EC2-PUBLIC-IP>` | Proxied (orange) |

### Step 4: Mine First Blocks
```bash
# Generate wallet and mine
opensy-cli createwallet "miner"
opensy-cli getnewaddress
opensy-cli generatetoaddress 100 <YOUR-ADDRESS>
```

### Step 5: Deploy DNS Seeder (Later)
```bash
# On seeder server:
cd /contrib/seeder/opensy-seeder
./dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net -p 5353
```

### Step 6: Update chainparams.cpp (After deployment)
```cpp
// In CMainParams constructor:
vSeeds.emplace_back("seed.opensy.net");
```

---

## 🔧 Technical Reference

### Network Parameters
| Parameter | Mainnet | Testnet |
|-----------|---------|---------|
| Port | 9633 | 19633 |
| RPC Port | 9632 | 19632 |
| Magic | 0x53594c4d | 0x53594c54 |
| Address Prefix | F (35) | f (95) |
| Bech32 HRP | syl | tsyl |

### Genesis Block
- **Hash:** `0000000727ee231c405685355f07629b06bfcb462cfa1ed7de868a6d9590ca8d`
- **Timestamp:** 1733616000 (Dec 8, 2024)
- **Message:** "Dec 8 2024 - Syria Liberated from Assad / سوريا حرة"
- **Difficulty:** 1.0

### Economics
- **Block Time:** 2 minutes
- **Initial Reward:** 10,000 SYL
- **Halving:** Every 1,050,000 blocks (~4 years)
- **Max Supply:** 21 billion SYL

---

## 📁 Key Files

| File | Purpose |
|------|---------|
| `/contrib/seeder/opensy-seeder/` | DNS seeder (compiled) |
| `/contrib/deploy/setup-node.sh` | Node setup automation |
| `/contrib/deploy/setup-dns-seeder.sh` | Seeder setup automation |
| `/contrib/deploy/docker/` | Docker deployment |
| `/docs/deployment/INFRASTRUCTURE_GUIDE.md` | Full deployment guide |
| `/src/kernel/chainparams.cpp` | Network parameters |
| `/build/bin/opensyd` | Compiled daemon |
| `/build/bin/opensy-cli` | Compiled CLI |
| `/build/bin/opensy-qt` | Compiled Qt wallet |

---

## 📊 Git Status

**Branch:** main  
**Last Commits:**
- `c2acabb` - fix: Update combine.pl port from 8333 to 9633
- `9e910f9` - chore: Complete seeder rebranding - remove Bitcoin references  
- `b026a08` - feat: Add DNS seeder and deployment infrastructure

**Status:** Clean (all changes committed and pushed)

---

## 🎯 Quick Resume Commands

```bash
# Check everything is ready
cd /Users/hamoudi/OpenSY
git status
ls -la contrib/seeder/opensy-seeder/dnsseed
./build/bin/opensyd --version

# Test daemon locally
./build/bin/opensyd -regtest -daemon
./build/bin/opensy-cli -regtest getblockchaininfo
./build/bin/opensy-cli -regtest stop
```

---

**مشروع سوريا المفتوحة - Syria's First Blockchain** 🇸🇾

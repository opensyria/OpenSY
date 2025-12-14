# 🔐 OpenSY Wallet Backup & Restore Guide

This guide explains how to backup and restore your OpenSY wallet.

## Backup Types

| Type | File | Contains | Restore Difficulty |
|------|------|----------|-------------------|
| **Wallet Backup** | `*.dat` | Full wallet database | Easy |
| **Descriptors** | `*.json` | Private keys + derivation paths | Medium |

Both can fully restore your funds. Keep both for redundancy.

---

## 📤 Creating Backups

### Method 1: Wallet Backup File (.dat)

```bash
# Using CLI
opensy-cli -rpcwallet=YOUR_WALLET backupwallet /path/to/backup.dat

# Example
opensy-cli -rpcwallet=founders-wallet backupwallet ~/Desktop/wallet-backup.dat
```

### Method 2: Export Descriptors (.json)

```bash
# Export with private keys (KEEP THIS SECURE!)
opensy-cli -rpcwallet=YOUR_WALLET listdescriptors true > wallet-descriptors.json
```

### Using GUI (OpenSY-Qt)

1. Open OpenSY-Qt
2. Go to **File** → **Backup Wallet...**
3. Choose location and save

---

## 📥 Restoring Wallets

### Method 1: From .dat Backup File

**Option A: Replace wallet directory**
```bash
# Stop the node first
opensy-cli stop

# Copy backup to wallet directory
# Linux/Mac:
cp /path/to/backup.dat ~/.opensy/wallets/restored-wallet/wallet.dat

# Start node
opensyd -daemon
```

**Option B: Using CLI (recommended)**
```bash
# Create new wallet and restore
opensy-cli restorewallet "restored-wallet" /path/to/backup.dat
```

### Method 2: From Descriptors JSON

```bash
# 1. Create a new blank descriptor wallet
opensy-cli createwallet "restored-wallet" false true true

# 2. Read your descriptors file and import
# The JSON file contains an array of descriptors like:
# {
#   "desc": "wpkh(xprv...)#checksum",
#   "timestamp": 1234567890,
#   ...
# }

# 3. Import each descriptor (example for one):
opensy-cli -rpcwallet=restored-wallet importdescriptors '[
  {
    "desc": "wpkh(xprv9s21ZrQH143K...)/*",
    "timestamp": "now",
    "active": true,
    "internal": false
  }
]'

# 4. Rescan blockchain
opensy-cli -rpcwallet=restored-wallet rescanblockchain
```

### Method 3: Using GUI

1. Open OpenSY-Qt
2. Go to **File** → **Open Wallet...**
3. Select your backup file

---

## 🔄 Full Restore Example

### Scenario: Lost computer, have descriptors backup

```bash
# 1. Install OpenSY on new computer
git clone https://github.com/opensyria/OpenSY.git
cd OpenSY && cmake -B build && cmake --build build

# 2. Start node and sync
./build/bin/opensyd -daemon -addnode=node1.opensy.net

# 3. Wait for sync (check progress)
./build/bin/opensy-cli getblockchaininfo

# 4. Create wallet and import descriptors
./build/bin/opensy-cli createwallet "recovered" false true true

# 5. Import your descriptors (from your backup JSON)
./build/bin/opensy-cli -rpcwallet=recovered importdescriptors '[...]'

# 6. Rescan to find transactions
./build/bin/opensy-cli -rpcwallet=recovered rescanblockchain

# 7. Check balance
./build/bin/opensy-cli -rpcwallet=recovered getbalance
```

---

## ⚠️ Important Security Notes

### DO:
- ✅ Store backups in multiple secure locations
- ✅ Encrypt backup files with a strong password
- ✅ Use offline/cold storage (USB drive in safe)
- ✅ Test restore process with small amount first
- ✅ Keep copies in different geographic locations

### DON'T:
- ❌ Store unencrypted backups in cloud storage
- ❌ Share backup files with anyone
- ❌ Keep only one copy
- ❌ Store backup on same device as wallet
- ❌ Email or message backup files

---

## 🔒 Encrypting Your Backup

### Using ZIP encryption (basic)
```bash
zip -e wallet-backup-encrypted.zip wallet-backup.dat wallet-descriptors.json
# Enter password when prompted
```

### Using GPG (stronger)
```bash
gpg -c wallet-backup.dat
# Creates wallet-backup.dat.gpg
```

### Decrypt later
```bash
gpg wallet-backup.dat.gpg
# Enter password
```

---

## 📋 Backup Checklist

- [ ] Created `.dat` backup file
- [ ] Exported descriptors `.json`
- [ ] Encrypted backup files
- [ ] Copied to USB drive
- [ ] Stored USB in secure location
- [ ] Copied to second location (safety deposit box, etc.)
- [ ] Tested restore process
- [ ] Deleted unencrypted copies from computer

---

## 🆘 Troubleshooting

### "Wallet not found"
```bash
# List available wallets
opensy-cli listwallets

# Load wallet
opensy-cli loadwallet "wallet-name"
```

### "Balance is 0 after restore"
```bash
# Rescan the blockchain
opensy-cli -rpcwallet=YOUR_WALLET rescanblockchain
```

### "Descriptor import failed"
- Check JSON format is valid
- Ensure checksum is correct
- Try importing one descriptor at a time

---

## 📞 Need Help?

- GitHub Issues: https://github.com/opensyria/OpenSY/issues
- Documentation: https://github.com/opensyria/OpenSY/tree/main/docs

---

**سوريا حرة** 🇸🇾

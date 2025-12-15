OpenSY
=====================================

<p align="center">
  <strong>🇸🇾 Syria's First Blockchain - Launched December 8, 2025</strong>
</p>

<p align="center">
  <em>"Dec 8 2024 - Syria Liberated from Assad / سوريا حرة"</em><br>
  — Genesis Block Message
</p>

---

## 🚀 Network Status: LIVE

| Metric | Value |
|--------|-------|
| **Status** | ✅ Mainnet Active |
| **Launch Date** | December 8, 2025 |
| **Seed Node** | `seed.opensyria.net` |
| **Block Explorer** | 🔍 [explorer.opensy.net](https://explorer.opensy.net) |
| **Website** | 🌐 [opensy.net](https://opensy.net) |

## Quick Start

```bash
# Clone the repository
git clone https://github.com/opensyria/OpenSY.git
cd OpenSY

# Build
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)

# Run a node
./build/bin/opensyd -daemon -addnode=node1.opensy.net

# Check sync status
./build/bin/opensy-cli getblockchaininfo
```

## What is OpenSY?

OpenSY (SYL) is a cryptocurrency forked from Bitcoin Core, created to support
the Syrian community with a modern, decentralized digital currency. It maintains
Bitcoin's proven security model while introducing Syria-specific customizations.

**Key Features:**
- **Currency:** SYL (Syrian Lira digital)
- **Address Prefix:** Mainnet addresses start with 'F' (Freedom)
- **Bech32 Prefix:** `syl1...` (native SegWit)
- **Network Port:** 9633 (based on Syria's country code +963)
- **Block Time:** ~2 minutes
- **Block Reward:** 10,000 SYL (halves every ~4 years)
- **Max Supply:** 21 Billion SYL
- **Genesis Date:** December 8, 2024 (Syria Liberation Day)
- **PoW Algorithm:** RandomX (CPU-friendly, ASIC-resistant) - from block 1

## Network Parameters

| Parameter | Mainnet | Testnet |
|-----------|---------|---------|
| Address Prefix | F | f |
| P2P Port | 9633 | 19633 |
| RPC Port | 9632 | 19632 |
| Bech32 Prefix | syl | tsyl |
| Block Time | ~2 min | ~2 min |

## Mining

### ⚡ RandomX - CPU-Friendly Mining from Day One

**OpenSY uses RandomX proof-of-work from block 1!**

| Block | Algorithm | Hardware |
|-------|-----------|----------|
| Genesis (Block 0) | SHA-256d | Bootstrap only |
| **Block 1+** | **RandomX** | **CPU** |

RandomX is an ASIC-resistant, CPU-optimized algorithm (used by Monero). This makes mining accessible to everyone with a regular computer from day one!

**Why RandomX from Block 1?**
- ✅ Fair launch - No ASIC/GPU advantage ever
- ✅ ASIC-resistant - No specialized hardware needed
- ✅ CPU-optimized - Your laptop can mine
- ✅ Truly decentralized - Levels the playing field from genesis
- ✅ Energy efficient - Lower power consumption

### Solo Mining (CPU)
```bash
# Create a wallet
./build/bin/opensy-cli createwallet "my-wallet"

# Get a mining address
./build/bin/opensy-cli getnewaddress "mining"

# Mine blocks (CPU mining)
./build/bin/opensy-cli generatetoaddress 10 ADDRESS
```

### CPU Mining Hardware
For optimal mining, consider high-core-count CPUs:

**Recommended Hardware:**
- AMD EPYC (64+ cores) - Best for RandomX
- Intel Xeon (32+ cores) - Good performance
- Any modern CPU with 8+ cores - Entry level

```bash
# Build and mine
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)
./build/bin/opensy-cli generatetoaddress 100 YOUR_ADDRESS
```

> **Note:** Block rewards require 100 confirmations (~3.3 hours) to mature.

## Documentation

- [Build Instructions](/doc/build-unix.md) - Linux/Unix
- [Build Instructions](/doc/build-osx.md) - macOS
- [Build Instructions](/doc/build-windows.md) - Windows
- [Launch Day Details](/docs/LAUNCH_DAY.md) - Network launch information
- [Infrastructure Guide](/docs/deployment/INFRASTRUCTURE_GUIDE.md) - Run your own node

## Connect

- **Website:** https://opensy.net
- **Explorer:** https://explorer.opensy.net
- **GitHub:** https://github.com/opensyria/OpenSY
- **DNS Seed:** seed.opensyria.net:9633
- **Primary Node:** 157.175.40.131:9633

## License

OpenSY is released under the terms of the MIT license. See [COPYING](COPYING) for more
information or see https://opensource.org/license/MIT.

---

**سوريا حرة** 🇸🇾

*For the people of Syria, by the people of Syria.*

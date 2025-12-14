# OpenSY RandomX - Technical Design Document

**Version:** 1.1  
**Date:** December 11, 2025  
**Status:** Implemented (Active from Block 1)  
**Author:** OpenSY Team

---

## Executive Summary

This document describes the technical implementation of OpenSY's RandomX
Proof-of-Work algorithm. OpenSY launched with RandomX active from block 1,
making it CPU-mineable and ASIC-resistant from day one. This democratizes 
mining access for Syrian users.

---

## Table of Contents

1. [Motivation](#motivation)
2. [Technical Overview](#technical-overview)
3. [Architecture](#architecture)
4. [Implementation Plan](#implementation-plan)
5. [Testing Strategy](#testing-strategy)
6. [Deployment Plan](#deployment-plan)
7. [Rollback Plan](#rollback-plan)
8. [Security Considerations](#security-considerations)

---

## Motivation

### Current Problem
- OpenSY uses SHA256d (same as Bitcoin)
- GPU/ASIC miners can easily dominate the network
- One miner currently controls ~100% of hashrate with GPU mining
- Average Syrians cannot compete without expensive hardware

### Solution
- Switch to RandomX (used by Monero)
- CPU-optimized, ASIC-resistant algorithm
- Levels the playing field for all miners
- Anyone with a computer can mine

### Benefits
| Aspect | Before (SHA256d) | After (RandomX) |
|--------|------------------|-----------------|
| Mining Hardware | GPU/ASIC required | Any CPU works |
| Barrier to Entry | High ($1000s) | Low (existing PC) |
| Decentralization | Low (few miners) | High (many miners) |
| Syrian Accessibility | Poor | Excellent |

---

## Technical Overview

### RandomX Specifications
- **Algorithm:** RandomX 1.1.10+
- **Hash Output:** 256 bits
- **Key:** Block hash at (height - 64), updated every 64 blocks
- **Mode:** Light mode for verification, Full mode for mining
- **Memory:** 256 MB (mining), 256 KB (verification)

### Fork Parameters
```cpp
// Consensus parameters (in src/consensus/params.h)
int nRandomXForkHeight;              // Block height to activate RandomX
int nRandomXKeyBlockInterval = 64;   // How often to update RandomX key
```

### Algorithm Selection Logic
```
if (block.height >= nRandomXForkHeight):
    use RandomX for PoW verification
else:
    use SHA256d for PoW verification (legacy blocks)
```

---

## Architecture

### Component Changes

```
src/
├── crypto/
│   └── randomx/           # NEW: RandomX library integration
│       ├── randomx.h
│       └── CMakeLists.txt
├── consensus/
│   └── params.h           # MODIFY: Add fork height parameter
├── pow.h                  # MODIFY: Add RandomX function declarations
├── pow.cpp                # MODIFY: Add RandomX validation logic
├── primitives/
│   └── block.cpp          # MODIFY: Add GetRandomXHash() method
├── miner.cpp              # MODIFY: Add RandomX mining support
├── rpc/
│   └── mining.cpp         # MODIFY: Update mining RPC for RandomX
└── test/
    └── pow_tests.cpp      # MODIFY: Add RandomX tests
```

### Class Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      Consensus::Params                       │
├─────────────────────────────────────────────────────────────┤
│ + nRandomXForkHeight: int                                   │
│ + nRandomXKeyBlockInterval: int                             │
│ + IsRandomXActive(height): bool                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        RandomXContext                        │
├─────────────────────────────────────────────────────────────┤
│ - m_cache: randomx_cache*                                   │
│ - m_vm: randomx_vm*                                         │
│ - m_keyBlockHash: uint256                                   │
├─────────────────────────────────────────────────────────────┤
│ + Initialize(keyBlockHash): void                            │
│ + CalculateHash(input): uint256                             │
│ + IsInitialized(): bool                                     │
│ + UpdateKey(newKeyBlockHash): void                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    CheckProofOfWork()                        │
├─────────────────────────────────────────────────────────────┤
│ if height >= nRandomXForkHeight:                            │
│     hash = RandomXContext.CalculateHash(blockHeader)        │
│ else:                                                       │
│     hash = SHA256d(blockHeader)                             │
│ return hash <= target                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation Plan

### Phase 1: Infrastructure (Day 1)

#### 1.1 Add RandomX Library
```cmake
# cmake/randomx.cmake
include(FetchContent)
FetchContent_Declare(
    randomx
    GIT_REPOSITORY https://github.com/tevador/RandomX.git
    GIT_TAG        v1.1.10
)
FetchContent_MakeAvailable(randomx)
```

#### 1.2 Update Consensus Parameters
```cpp
// src/consensus/params.h
struct Params {
    // ... existing fields ...
    
    /** RandomX Hard Fork Parameters */
    int nRandomXForkHeight{60000};           // Activate at block 60000
    int nRandomXKeyBlockInterval{64};        // Key changes every 64 blocks
    
    /** Check if RandomX is active at given height */
    bool IsRandomXActive(int height) const {
        return height >= nRandomXForkHeight;
    }
    
    /** Get the key block height for a given block height */
    int GetRandomXKeyBlockHeight(int height) const {
        return (height / nRandomXKeyBlockInterval) * nRandomXKeyBlockInterval - nRandomXKeyBlockInterval;
    }
};
```

### Phase 2: Core Implementation (Day 1-2)

#### 2.1 RandomX Context Manager
```cpp
// src/crypto/randomx_context.h
#ifndef OPENSY_RANDOMX_CONTEXT_H
#define OPENSY_RANDOMX_CONTEXT_H

#include <uint256.h>
#include <randomx.h>
#include <mutex>
#include <memory>

/**
 * Thread-safe RandomX context manager for hash calculation.
 * 
 * RandomX requires initialization with a "key" (block hash) that changes
 * periodically. This class manages the cache and VM lifecycle.
 */
class RandomXContext {
private:
    randomx_cache* m_cache{nullptr};
    randomx_vm* m_vm{nullptr};
    uint256 m_keyBlockHash;
    mutable std::mutex m_mutex;
    bool m_initialized{false};
    
public:
    RandomXContext() = default;
    ~RandomXContext();
    
    // Non-copyable
    RandomXContext(const RandomXContext&) = delete;
    RandomXContext& operator=(const RandomXContext&) = delete;
    
    /**
     * Initialize or reinitialize with a new key block hash.
     * @param keyBlockHash Hash of the block used as RandomX key
     * @return true if initialization succeeded
     */
    bool Initialize(const uint256& keyBlockHash);
    
    /**
     * Calculate RandomX hash of input data.
     * @param input Raw bytes to hash (typically serialized block header)
     * @return 256-bit hash result
     */
    uint256 CalculateHash(const std::vector<unsigned char>& input);
    
    /**
     * Check if context is initialized and ready.
     */
    bool IsInitialized() const;
    
    /**
     * Get the current key block hash.
     */
    uint256 GetKeyBlockHash() const;
};

/** Global RandomX context for validation (light mode) */
extern std::unique_ptr<RandomXContext> g_randomx_context;

/** Initialize global RandomX context */
void InitRandomXContext();

/** Shutdown global RandomX context */
void ShutdownRandomXContext();

#endif // OPENSY_RANDOMX_CONTEXT_H
```

#### 2.2 Modify pow.cpp
```cpp
// Addition to src/pow.cpp

#include <crypto/randomx_context.h>

/**
 * Calculate RandomX hash for a block header.
 * 
 * @param header Block header to hash
 * @param keyBlockHash Hash of the key block (height - 64)
 * @return RandomX hash of the block header
 */
uint256 CalculateRandomXHash(const CBlockHeader& header, const uint256& keyBlockHash)
{
    // Serialize block header
    CDataStream ss(SER_NETWORK, PROTOCOL_VERSION);
    ss << header;
    std::vector<unsigned char> data(ss.begin(), ss.end());
    
    // Ensure context is initialized with correct key
    if (!g_randomx_context || g_randomx_context->GetKeyBlockHash() != keyBlockHash) {
        if (!g_randomx_context) {
            g_randomx_context = std::make_unique<RandomXContext>();
        }
        g_randomx_context->Initialize(keyBlockHash);
    }
    
    return g_randomx_context->CalculateHash(data);
}

/**
 * Get the key block hash for RandomX at a given height.
 * Key changes every 64 blocks.
 */
uint256 GetRandomXKeyBlockHash(int height, const CBlockIndex* pindex)
{
    int keyHeight = (height / 64) * 64 - 64;
    if (keyHeight < 0) keyHeight = 0;
    
    const CBlockIndex* keyBlock = pindex;
    while (keyBlock && keyBlock->nHeight > keyHeight) {
        keyBlock = keyBlock->pprev;
    }
    
    return keyBlock ? keyBlock->GetBlockHash() : uint256();
}

bool CheckProofOfWork(uint256 hash, unsigned int nBits, const Consensus::Params& params)
{
    if (EnableFuzzDeterminism()) return (hash.data()[31] & 0x80) == 0;
    return CheckProofOfWorkImpl(hash, nBits, params);
}

/**
 * Enhanced CheckProofOfWork that handles both SHA256d and RandomX.
 * 
 * @param header Block header to verify
 * @param nBits Target in compact form
 * @param params Consensus parameters
 * @param height Block height (needed to determine which algorithm)
 * @param pindexPrev Previous block index (needed for RandomX key)
 * @return true if proof of work is valid
 */
bool CheckProofOfWorkEx(const CBlockHeader& header, unsigned int nBits, 
                        const Consensus::Params& params, int height,
                        const CBlockIndex* pindexPrev)
{
    uint256 hash;
    
    if (params.IsRandomXActive(height)) {
        // RandomX: Need key block hash
        uint256 keyBlockHash = GetRandomXKeyBlockHash(height, pindexPrev);
        hash = CalculateRandomXHash(header, keyBlockHash);
    } else {
        // Legacy SHA256d
        hash = header.GetHash();
    }
    
    return CheckProofOfWorkImpl(hash, nBits, params);
}
```

### Phase 3: Mining Support (Day 2)

#### 3.1 Update Miner
```cpp
// Modifications to src/miner.cpp

/**
 * Mine a block using appropriate PoW algorithm.
 */
bool ScanHash(const CBlockHeader& header, uint32_t& nNonce, uint256& hash,
              bool useRandomX, const uint256& randomxKey)
{
    CBlockHeader headerCopy = header;
    
    while (true) {
        headerCopy.nNonce = nNonce;
        
        if (useRandomX) {
            hash = CalculateRandomXHash(headerCopy, randomxKey);
        } else {
            hash = headerCopy.GetHash();
        }
        
        // Check if we found a valid hash
        if (UintToArith256(hash) <= target) {
            return true;
        }
        
        nNonce++;
        if (nNonce == 0) return false; // Overflow
    }
}
```

---

## Testing Strategy

### Unit Tests

```cpp
// src/test/pow_tests.cpp - NEW TESTS

BOOST_AUTO_TEST_CASE(randomx_basic_hash)
{
    // Test that RandomX produces consistent hashes
    RandomXContext ctx;
    uint256 key = uint256S("0x1234...");
    ctx.Initialize(key);
    
    std::vector<unsigned char> input = {0x01, 0x02, 0x03};
    uint256 hash1 = ctx.CalculateHash(input);
    uint256 hash2 = ctx.CalculateHash(input);
    
    BOOST_CHECK_EQUAL(hash1, hash2);
}

BOOST_AUTO_TEST_CASE(randomx_different_keys_different_hashes)
{
    // Same input with different keys should produce different hashes
    RandomXContext ctx;
    
    uint256 key1 = uint256S("0x1111...");
    uint256 key2 = uint256S("0x2222...");
    std::vector<unsigned char> input = {0x01, 0x02, 0x03};
    
    ctx.Initialize(key1);
    uint256 hash1 = ctx.CalculateHash(input);
    
    ctx.Initialize(key2);
    uint256 hash2 = ctx.CalculateHash(input);
    
    BOOST_CHECK(hash1 != hash2);
}

BOOST_AUTO_TEST_CASE(fork_activation_sha256d_before)
{
    // Blocks before fork height should use SHA256d
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::REGTEST);
    auto& params = chainParams->GetConsensus();
    
    // Height before fork
    int height = params.nRandomXForkHeight - 1;
    BOOST_CHECK(!params.IsRandomXActive(height));
}

BOOST_AUTO_TEST_CASE(fork_activation_randomx_after)
{
    // Blocks at and after fork height should use RandomX
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::REGTEST);
    auto& params = chainParams->GetConsensus();
    
    // Height at fork
    int height = params.nRandomXForkHeight;
    BOOST_CHECK(params.IsRandomXActive(height));
    
    // Height after fork
    BOOST_CHECK(params.IsRandomXActive(height + 1000));
}

BOOST_AUTO_TEST_CASE(randomx_pow_validation)
{
    // Test full PoW validation with RandomX
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::REGTEST);
    auto& params = chainParams->GetConsensus();
    
    // Create a block header
    CBlockHeader header;
    header.nVersion = 1;
    header.nTime = 1234567890;
    header.nBits = 0x207fffff; // Regtest difficulty
    header.nNonce = 0;
    
    // Mine until we find valid PoW
    uint256 keyBlockHash = uint256S("0x0000...");
    while (true) {
        uint256 hash = CalculateRandomXHash(header, keyBlockHash);
        if (CheckProofOfWorkImpl(hash, header.nBits, params)) {
            break;
        }
        header.nNonce++;
        BOOST_REQUIRE(header.nNonce < 1000000); // Safety limit
    }
    
    // Verify the mined block passes validation
    BOOST_CHECK(CheckProofOfWorkEx(header, header.nBits, params, 
                params.nRandomXForkHeight, nullptr));
}
```

### Integration Tests

```python
# test/functional/feature_randomx_fork.py

#!/usr/bin/env python3
"""Test RandomX hard fork activation."""

from test_framework.test_framework import OpenSYTestFramework
from test_framework.util import assert_equal

class RandomXForkTest(OpenSYTestFramework):
    def set_test_params(self):
        self.num_nodes = 2
        self.setup_clean_chain = True
        # Set low fork height for testing
        self.extra_args = [["-randomxforkheight=100"], ["-randomxforkheight=100"]]

    def run_test(self):
        self.log.info("Testing SHA256d mining before fork...")
        # Mine blocks before fork height
        self.generate(self.nodes[0], 99)
        assert_equal(self.nodes[0].getblockcount(), 99)
        
        self.log.info("Testing RandomX mining at fork...")
        # This block should use RandomX
        self.generate(self.nodes[0], 1)
        assert_equal(self.nodes[0].getblockcount(), 100)
        
        self.log.info("Testing RandomX mining after fork...")
        self.generate(self.nodes[0], 10)
        assert_equal(self.nodes[0].getblockcount(), 110)
        
        self.log.info("Testing chain sync between nodes...")
        self.sync_all()
        assert_equal(self.nodes[0].getblockcount(), self.nodes[1].getblockcount())

if __name__ == '__main__':
    RandomXForkTest().main()
```

---

## Deployment Plan

### Timeline

| Date | Action |
|------|--------|
| Day 0 | Code complete, all tests pass |
| Day 1 | Release v31.0.0-rc1 (release candidate) |
| Day 1 | Announce fork on website, GitHub, social media |
| Day 2 | Release v31.0.0 final |
| Day 2 | Update AWS node to v31.0.0 |
| Day 3-5 | Grace period for other nodes to upgrade |
| Block 60000 | Fork activates (~2-3 days from announcement) |

### Announcement Template

```markdown
# OpenSY: RandomX CPU Mining from Day One

## What's Different?
OpenSY launched with RandomX proof-of-work active from block 1.
Only the genesis block (block 0) uses SHA256d for bootstrap purposes.

## Why RandomX?
- Makes mining accessible to everyone with a CPU
- No expensive GPU/ASIC hardware needed
- Aligns with OpenSY's mission of accessibility

## For Miners
- CPU mining works with any modern processor
- Use `generatetoaddress` command to mine
- No special hardware required

## Download
- GitHub: https://github.com/opensyria/OpenSY/releases
```

---

## Rollback Plan

If critical issues are discovered:

1. **Before fork activation:** Simply don't upgrade nodes
2. **After fork activation:** 
   - Release emergency patch to increase fork height
   - Nodes on old chain continue as-is
   - Some blocks may be orphaned

---

## Security Considerations

### Risks

1. **RandomX vulnerabilities:** Mitigated by using stable v1.1.10 release
2. **Implementation bugs:** Mitigated by comprehensive testing
3. **51% attack during transition:** Mitigated by short upgrade window
4. **Botnet mining:** Inherent RandomX risk, acceptable tradeoff

### Mitigations

- Extensive unit and integration testing
- Code review before merge
- Testnet deployment first (if time permits)
- Monitoring during fork activation

---

## References

- [RandomX Specification](https://github.com/tevador/RandomX/blob/master/doc/specs.md)
- [Monero RandomX Implementation](https://github.com/monero-project/monero)
- [OpenSY Source Code](https://github.com/opensyria/OpenSY)

---

## Appendix A: File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `CMakeLists.txt` | Modify | Add RandomX dependency |
| `cmake/randomx.cmake` | New | RandomX build configuration |
| `src/consensus/params.h` | Modify | Add fork parameters |
| `src/crypto/randomx_context.h` | New | RandomX context manager header |
| `src/crypto/randomx_context.cpp` | New | RandomX context manager impl |
| `src/pow.h` | Modify | Add new function declarations |
| `src/pow.cpp` | Modify | Add RandomX validation logic |
| `src/miner.cpp` | Modify | Add RandomX mining support |
| `src/rpc/mining.cpp` | Modify | Update RPC for algorithm info |
| `src/chainparams.cpp` | Modify | Set fork height for each network |
| `src/test/pow_tests.cpp` | Modify | Add RandomX unit tests |
| `test/functional/feature_randomx_fork.py` | New | Integration tests |

---

*Document Version: 1.0 - Initial Draft*

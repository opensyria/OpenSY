# OpenSY Emergency PoW Fallback: Argon2id

**Version:** 1.0  
**Date:** December 21, 2025  
**Status:** Dormant (Emergency Only)

---

## Overview

OpenSY includes a dormant emergency fallback proof-of-work algorithm: **Argon2id**.

This mechanism is designed to protect the network if RandomX is ever compromised (cryptographic break, critical implementation vulnerability, or novel attack vector). The fallback is **not active by default** and can only be activated via a consensus hard fork.

---

## Why Argon2id?

| Criteria | Argon2id | Alternative Considered |
|----------|----------|------------------------|
| **ASIC Resistance** | ✅ Memory-hard | SHA256d has ASICs |
| **CPU Friendly** | ✅ Designed for CPUs | scrypt has GPU ASICs |
| **Audit Quality** | ✅ PHC Winner 2015 | yespower less reviewed |
| **Side-Channel Resistance** | ✅ id variant | CryptoNight deprecated |
| **Library Support** | ✅ libsodium, widespread | Custom implementations risky |
| **Complexity** | ✅ Simpler than RandomX | Fewer attack surfaces |

### Argon2id Properties

- **Memory**: Configurable (default 2GB, matching RandomX)
- **Time Cost**: 1 iteration (tuned for ~100ms per hash)
- **Parallelism**: 1 (prevents GPU optimization)
- **Output**: 256-bit hash

---

## Activation Mechanism

### Default State: DORMANT

```cpp
consensus.nArgon2EmergencyHeight = -1;  // Never active
```

The fallback is dormant by default. RandomX remains the active PoW algorithm.

### Emergency Activation

If RandomX is compromised, the OpenSY developers would:

1. **Assess the threat** (cryptographic break, CVE, etc.)
2. **Coordinate with miners** via announcement channels
3. **Set activation height** in new release:
   ```cpp
   consensus.nArgon2EmergencyHeight = <BLOCK_HEIGHT>;
   ```
4. **Release emergency update** with mandatory upgrade notice
5. **Hard fork activates** at specified height

### Algorithm Selection Logic

```
height == 0                              → SHA256d (genesis)
height >= 1 && nArgon2EmergencyHeight < 0  → RandomX
height >= 1 && height >= nArgon2EmergencyHeight → Argon2id
```

---

## Technical Implementation

### Files

| File | Purpose |
|------|---------|
| [src/crypto/argon2_context.h](../../src/crypto/argon2_context.h) | Argon2 context header |
| [src/crypto/argon2_context.cpp](../../src/crypto/argon2_context.cpp) | Argon2 implementation |
| [src/consensus/params.h](../../src/consensus/params.h) | Algorithm selection |
| [src/pow.cpp](../../src/pow.cpp) | PoW validation integration |
| [src/test/argon2_fallback_tests.cpp](../../src/test/argon2_fallback_tests.cpp) | Unit tests |

### Consensus Parameters

```cpp
// In Consensus::Params
int nArgon2EmergencyHeight{-1};       // Height for activation (-1 = never)
uint32_t nArgon2MemoryCost{1 << 21};  // Memory in KiB (2GB)
uint32_t nArgon2TimeCost{1};          // Iterations
uint32_t nArgon2Parallelism{1};       // Threads
uint256 powLimitArgon2;               // Minimum difficulty
```

### Algorithm Enum

```cpp
enum class PowAlgorithm {
    SHA256D,    // Genesis block
    RANDOMX,    // Primary algorithm (block 1+)
    ARGON2ID    // Emergency fallback
};
```

---

## Security Considerations

### Why Not Pre-Announce the Fallback Height?

Pre-announcing an activation height would:
- Allow attackers to prepare optimized hardware
- Create uncertainty about which chain is canonical
- Enable manipulation of the transition

The fallback is intentionally **dormant** until needed.

### What Triggers Activation?

| Scenario | Response |
|----------|----------|
| RandomX cryptographic break | Immediate emergency fork |
| Critical CVE in RandomX implementation | Emergency fork within days |
| Theoretical weakness discovered | Planned upgrade with lead time |
| 51% attack (not algo-related) | No algo change needed |

### Difficulty Reset

At emergency activation, difficulty resets to `powLimitArgon2` to allow miners to immediately participate with the new algorithm.

---

## Dependencies

### libsodium (Recommended)

For production deployments, link against libsodium for optimized Argon2id:

```bash
# macOS
brew install libsodium

# Ubuntu/Debian
apt install libsodium-dev

# Build with libsodium
cmake -B build -DCMAKE_BUILD_TYPE=Release
# libsodium is auto-detected via pkg-config
```

If libsodium is not available, a reference implementation stub is used (suitable for testing, not production).

---

## Testing

### Unit Tests

```bash
# Run Argon2 fallback tests
./build/bin/test_opensy --run_test=argon2_fallback_tests

# Run all tests
./build/bin/test_opensy
```

### Regtest Testing

Test emergency activation on regtest:

```bash
# Start regtest with emergency activation at height 100
./opensyd -regtest -randomxforkheight=1 -argon2emergencyheight=100

# Mine past activation height
./opensy-cli -regtest generatetoaddress 150 <address>

# Verify Argon2id is active
./opensy-cli -regtest getblockchaininfo
```

---

## FAQ

### Q: Is Argon2id currently active?

**No.** It is dormant. RandomX is the active PoW algorithm. Argon2id only activates in an emergency.

### Q: Will my miner work with Argon2id?

If Argon2id is activated, miners will need to update their software. The new version will include Argon2id mining support.

### Q: Why not use multiple algorithms like DigiByte?

Multi-algo introduces complexity and potential attack vectors (algo-hopping, difficulty manipulation). A single primary algorithm with a dormant fallback is simpler and more secure.

### Q: What if Argon2id is also compromised?

If both RandomX and Argon2id were compromised simultaneously (extremely unlikely), the network would need a more fundamental upgrade. The fallback buys time for such a response.

---

## References

- [Argon2 Specification (RFC 9106)](https://datatracker.ietf.org/doc/html/rfc9106)
- [Password Hashing Competition](https://password-hashing.net/)
- [libsodium Documentation](https://doc.libsodium.org/)
- [RandomX Specification](https://github.com/tevador/RandomX)

---

*سوريا حرة* 🇸🇾

// Copyright (c) 2025 The OpenSY developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#include <crypto/argon2_context.h>
#include <consensus/params.h>
#include <logging.h>
#include <streams.h>
#include <util/check.h>

// Argon2 reference implementation
// TODO: Replace with libsodium for production (better optimized)
// For now, we include a minimal implementation stub
// Full implementation requires: https://github.com/P-H-C/phc-winner-argon2
#ifdef HAVE_LIBSODIUM
#include <sodium.h>
#define USE_LIBSODIUM 1
#else
// Stub for when libsodium is not available
// In production, this should be replaced with actual Argon2 reference impl
#define USE_LIBSODIUM 0
#include <crypto/sha256.h> // Fallback for development/testing
#endif

#include <stdexcept>

std::unique_ptr<Argon2Context> g_argon2_context;

Argon2Context::Argon2Context(uint32_t memory_cost, uint32_t time_cost, uint32_t parallelism)
    : m_memory_cost(memory_cost), m_time_cost(time_cost), m_parallelism(parallelism)
{
    // Validate parameters
    if (memory_cost < 8) {
        throw std::invalid_argument("Argon2 memory_cost must be at least 8 KiB");
    }
    if (time_cost < 1) {
        throw std::invalid_argument("Argon2 time_cost must be at least 1");
    }
    if (parallelism < 1) {
        throw std::invalid_argument("Argon2 parallelism must be at least 1");
    }

#if USE_LIBSODIUM
    if (sodium_init() < 0) {
        throw std::runtime_error("Failed to initialize libsodium");
    }
#endif

    m_initialized = true;

    LogPrintf("Argon2Context: Initialized with memory=%u KiB, time=%u, parallelism=%u\n",
              m_memory_cost, m_time_cost, m_parallelism);
}

uint256 Argon2Context::CalculateHash(const std::vector<unsigned char>& input,
                                      const uint256& salt) const
{
    return CalculateHash(input.data(), input.size(), salt);
}

uint256 Argon2Context::CalculateHash(const unsigned char* data, size_t len,
                                      const uint256& salt) const
{
    LOCK(m_mutex);

    if (!m_initialized) {
        throw std::runtime_error("Argon2 context not initialized");
    }

    // Limit input size to prevent DoS
    static constexpr size_t ARGON2_MAX_INPUT_SIZE = 4 * 1024 * 1024; // 4MB
    if (len > ARGON2_MAX_INPUT_SIZE) {
        throw std::runtime_error("Argon2 input exceeds maximum size");
    }

    uint256 result;

#if USE_LIBSODIUM
    // Use libsodium's Argon2id implementation
    // crypto_pwhash with ALG_ARGON2ID13
    int ret = crypto_pwhash(
        result.begin(),                           // output
        HASH_LENGTH,                              // output length
        reinterpret_cast<const char*>(data),      // password (block header)
        len,                                      // password length
        salt.begin(),                             // salt (prev block hash)
        m_time_cost,                              // opslimit (iterations)
        static_cast<size_t>(m_memory_cost) * 1024,// memlimit (bytes)
        crypto_pwhash_ALG_ARGON2ID13              // algorithm
    );

    if (ret != 0) {
        throw std::runtime_error("Argon2id hash calculation failed");
    }
#else
    // DEVELOPMENT/TESTING FALLBACK
    // This is NOT the real Argon2 - just a placeholder for compilation
    // In production, libsodium or argon2 reference must be linked
    //
    // WARNING: Do not use this fallback for actual PoW validation!
    //
    LogPrintf("WARNING: Using SHA256 fallback instead of Argon2id - FOR TESTING ONLY\n");

    // Combine input with salt and hash with SHA256 (not memory-hard!)
    CSHA256 hasher;
    hasher.Write(data, len);
    hasher.Write(salt.begin(), 32);
    // Add parameters to make it deterministic based on config
    hasher.Write(reinterpret_cast<const unsigned char*>(&m_memory_cost), sizeof(m_memory_cost));
    hasher.Write(reinterpret_cast<const unsigned char*>(&m_time_cost), sizeof(m_time_cost));
    hasher.Finalize(result.begin());
#endif

    return result;
}

uint256 Argon2Context::CalculateBlockHash(const CBlockHeader& header) const
{
    // Serialize block header
    DataStream ss{};
    ss << header;

    // Use hashPrevBlock as salt for Argon2
    // This ensures each block has a unique salt, preventing precomputation
    return CalculateHash(
        reinterpret_cast<const unsigned char*>(ss.data()),
        ss.size(),
        header.hashPrevBlock
    );
}

bool Argon2Context::IsInitialized() const
{
    LOCK(m_mutex);
    return m_initialized;
}

void InitArgon2Context(uint32_t memory_cost, uint32_t time_cost, uint32_t parallelism)
{
    if (!g_argon2_context) {
        g_argon2_context = std::make_unique<Argon2Context>(
            memory_cost, time_cost, parallelism);
    }
}

uint256 CalculateArgon2Hash(const CBlockHeader& header, const Consensus::Params& params)
{
    // Lazily initialize global context
    if (!g_argon2_context) {
        InitArgon2Context(
            params.nArgon2MemoryCost,
            params.nArgon2TimeCost,
            params.nArgon2Parallelism
        );
    }

    return g_argon2_context->CalculateBlockHash(header);
}

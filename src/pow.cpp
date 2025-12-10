// Copyright (c) 2009-2010 Qirsh Nakamoto
// Copyright (c) 2009-2022 The OpenSyria Core developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

// TODO [SECURITY - SHA256d CONSIDERATIONS]:
// OpenSyria uses SHA256d (same as Bitcoin) for proof-of-work. This creates both
// advantages and risks:
//
// ADVANTAGES:
// - Battle-tested algorithm with 15+ years of security
// - Existing ASIC mining hardware can be used immediately
// - Well-understood difficulty adjustment behavior
//
// RISKS:
// - Bitcoin has ~500+ EH/s; a fraction redirected could overwhelm OpenSyria
// - NiceHash and similar services enable easy hashrate rental attacks
// - 51% attacks become economically viable once SYL has exchange value
//
// MITIGATIONS (implement these operationally):
// 1. Partner with mining pools early to build legitimate hashrate
// 2. Recommend exchanges require 50-100 confirmations for deposits
// 3. Monitor for sudden hashrate spikes (potential attack indicator)
// 4. Update nMinimumChainWork frequently during first year
// 5. Consider merge-mining with Bitcoin in future (can be soft-forked in)
// 6. Implement alerting system for abnormal block times or reorgs
//
// TODO [FUTURE ENHANCEMENT]: Consider implementing merge-mining support
// This would allow Bitcoin miners to mine OpenSyria "for free", dramatically
// increasing security. See namecoin/namecoin-core for reference implementation.
//
// NOTE [RANDOMX HARD FORK]:
// At block height nRandomXForkHeight (default 57500), OpenSyria switches from
// SHA256d to RandomX proof-of-work. This makes mining CPU-friendly and
// ASIC-resistant, democratizing mining for all participants.

#include <pow.h>

#include <arith_uint256.h>
#include <chain.h>
#include <crypto/randomx_context.h>
#include <primitives/block.h>
#include <streams.h>
#include <uint256.h>
#include <util/check.h>

unsigned int GetNextWorkRequired(const CBlockIndex* pindexLast, const CBlockHeader *pblock, const Consensus::Params& params)
{
    assert(pindexLast != nullptr);
    
    // Use different powLimit based on whether we're in RandomX territory
    int nextHeight = pindexLast->nHeight + 1;
    const uint256& activePowLimit = params.GetRandomXPowLimit(nextHeight);
    unsigned int nProofOfWorkLimit = UintToArith256(activePowLimit).GetCompact();

    // At the RandomX fork height, reset to minimum difficulty for the new algorithm
    if (nextHeight == params.nRandomXForkHeight) {
        return nProofOfWorkLimit;
    }

    // Only change once per difficulty adjustment interval
    if ((pindexLast->nHeight+1) % params.DifficultyAdjustmentInterval() != 0)
    {
        if (params.fPowAllowMinDifficultyBlocks)
        {
            // Special difficulty rule for testnet:
            // If the new block's timestamp is more than 2* 10 minutes
            // then it MUST be a min-difficulty block.
            if (pblock->GetBlockTime() > pindexLast->GetBlockTime() + params.nPowTargetSpacing*2)
                return nProofOfWorkLimit;
            else
            {
                // Return the last non-special-min-difficulty-rules-block
                const CBlockIndex* pindex = pindexLast;
                while (pindex->pprev && pindex->nHeight % params.DifficultyAdjustmentInterval() != 0 && pindex->nBits == nProofOfWorkLimit)
                    pindex = pindex->pprev;
                return pindex->nBits;
            }
        }
        return pindexLast->nBits;
    }

    // Go back by what we want to be 14 days worth of blocks
    int nHeightFirst = pindexLast->nHeight - (params.DifficultyAdjustmentInterval()-1);
    assert(nHeightFirst >= 0);
    const CBlockIndex* pindexFirst = pindexLast->GetAncestor(nHeightFirst);
    assert(pindexFirst);

    return CalculateNextWorkRequired(pindexLast, pindexFirst->GetBlockTime(), params);
}

unsigned int CalculateNextWorkRequired(const CBlockIndex* pindexLast, int64_t nFirstBlockTime, const Consensus::Params& params)
{
    if (params.fPowNoRetargeting)
        return pindexLast->nBits;

    // Limit adjustment step
    int64_t nActualTimespan = pindexLast->GetBlockTime() - nFirstBlockTime;
    if (nActualTimespan < params.nPowTargetTimespan/4)
        nActualTimespan = params.nPowTargetTimespan/4;
    if (nActualTimespan > params.nPowTargetTimespan*4)
        nActualTimespan = params.nPowTargetTimespan*4;

    // Use appropriate powLimit based on height (SHA256d vs RandomX)
    int nextHeight = pindexLast->nHeight + 1;
    const arith_uint256 bnPowLimit = UintToArith256(params.GetRandomXPowLimit(nextHeight));
    arith_uint256 bnNew;

    // Special difficulty rule for Testnet4
    if (params.enforce_BIP94) {
        // Here we use the first block of the difficulty period. This way
        // the real difficulty is always preserved in the first block as
        // it is not allowed to use the min-difficulty exception.
        int nHeightFirst = pindexLast->nHeight - (params.DifficultyAdjustmentInterval()-1);
        const CBlockIndex* pindexFirst = pindexLast->GetAncestor(nHeightFirst);
        bnNew.SetCompact(pindexFirst->nBits);
    } else {
        bnNew.SetCompact(pindexLast->nBits);
    }

    bnNew *= nActualTimespan;
    bnNew /= params.nPowTargetTimespan;

    if (bnNew > bnPowLimit)
        bnNew = bnPowLimit;

    return bnNew.GetCompact();
}

// Check that on difficulty adjustments, the new difficulty does not increase
// or decrease beyond the permitted limits.
bool PermittedDifficultyTransition(const Consensus::Params& params, int64_t height, uint32_t old_nbits, uint32_t new_nbits)
{
    if (params.fPowAllowMinDifficultyBlocks) return true;

    if (height % params.DifficultyAdjustmentInterval() == 0) {
        int64_t smallest_timespan = params.nPowTargetTimespan/4;
        int64_t largest_timespan = params.nPowTargetTimespan*4;

        const arith_uint256 pow_limit = UintToArith256(params.powLimit);
        arith_uint256 observed_new_target;
        observed_new_target.SetCompact(new_nbits);

        // Calculate the largest difficulty value possible:
        arith_uint256 largest_difficulty_target;
        largest_difficulty_target.SetCompact(old_nbits);
        largest_difficulty_target *= largest_timespan;
        largest_difficulty_target /= params.nPowTargetTimespan;

        if (largest_difficulty_target > pow_limit) {
            largest_difficulty_target = pow_limit;
        }

        // Round and then compare this new calculated value to what is
        // observed.
        arith_uint256 maximum_new_target;
        maximum_new_target.SetCompact(largest_difficulty_target.GetCompact());
        if (maximum_new_target < observed_new_target) return false;

        // Calculate the smallest difficulty value possible:
        arith_uint256 smallest_difficulty_target;
        smallest_difficulty_target.SetCompact(old_nbits);
        smallest_difficulty_target *= smallest_timespan;
        smallest_difficulty_target /= params.nPowTargetTimespan;

        if (smallest_difficulty_target > pow_limit) {
            smallest_difficulty_target = pow_limit;
        }

        // Round and then compare this new calculated value to what is
        // observed.
        arith_uint256 minimum_new_target;
        minimum_new_target.SetCompact(smallest_difficulty_target.GetCompact());
        if (minimum_new_target > observed_new_target) return false;
    } else if (old_nbits != new_nbits) {
        return false;
    }
    return true;
}

// Bypasses the actual proof of work check during fuzz testing with a simplified validation checking whether
// the most significant bit of the last byte of the hash is set.
bool CheckProofOfWork(uint256 hash, unsigned int nBits, const Consensus::Params& params)
{
    if (EnableFuzzDeterminism()) return (hash.data()[31] & 0x80) == 0;
    return CheckProofOfWorkImpl(hash, nBits, params);
}

std::optional<arith_uint256> DeriveTarget(unsigned int nBits, const uint256 pow_limit)
{
    bool fNegative;
    bool fOverflow;
    arith_uint256 bnTarget;

    bnTarget.SetCompact(nBits, &fNegative, &fOverflow);

    // Check range
    if (fNegative || bnTarget == 0 || fOverflow || bnTarget > UintToArith256(pow_limit))
        return {};

    return bnTarget;
}

bool CheckProofOfWorkImpl(uint256 hash, unsigned int nBits, const Consensus::Params& params)
{
    auto bnTarget{DeriveTarget(nBits, params.powLimit)};
    if (!bnTarget) return false;

    // Check proof of work matches claimed amount
    if (UintToArith256(hash) > bnTarget)
        return false;

    return true;
}

// Height-aware version that uses appropriate powLimit for SHA256d vs RandomX
bool CheckProofOfWorkImpl(uint256 hash, unsigned int nBits, int height, const Consensus::Params& params)
{
    const uint256& activePowLimit = params.GetRandomXPowLimit(height);
    auto bnTarget{DeriveTarget(nBits, activePowLimit)};
    if (!bnTarget) return false;

    // Check proof of work matches claimed amount
    if (UintToArith256(hash) > bnTarget)
        return false;

    return true;
}

// =============================================================================
// RANDOMX PROOF-OF-WORK FUNCTIONS
// =============================================================================

uint256 GetRandomXKeyBlockHash(int height, const CBlockIndex* pindex, const Consensus::Params& params)
{
    int keyHeight = params.GetRandomXKeyBlockHeight(height);

    // For early blocks (before we have enough history), use genesis
    if (keyHeight < 0) {
        keyHeight = 0;
    }

    // Traverse back to the key block
    const CBlockIndex* keyBlock = pindex;
    while (keyBlock && keyBlock->nHeight > keyHeight) {
        keyBlock = keyBlock->pprev;
    }

    // If we couldn't find the key block, return empty hash
    if (!keyBlock || keyBlock->nHeight != keyHeight) {
        return uint256();
    }

    return keyBlock->GetBlockHash();
}

uint256 CalculateRandomXHash(const CBlockHeader& header, const uint256& keyBlockHash)
{
    // Ensure global context exists
    if (!g_randomx_context) {
        g_randomx_context = std::make_unique<RandomXContext>();
    }

    // Initialize or update context if key changed
    if (g_randomx_context->GetKeyBlockHash() != keyBlockHash) {
        if (!g_randomx_context->Initialize(keyBlockHash)) {
            // Initialization failed - return max hash (will always fail PoW check)
            return uint256{"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"};
        }
    }

    // Serialize block header
    DataStream ss{};
    ss << header;

    // Calculate and return RandomX hash
    return g_randomx_context->CalculateHash(
        reinterpret_cast<const unsigned char*>(ss.data()), ss.size());
}

bool CheckProofOfWorkAtHeight(const CBlockHeader& header, int height, const CBlockIndex* pindex, const Consensus::Params& params)
{
    if (params.IsRandomXActive(height)) {
        // RandomX proof-of-work for blocks at or after fork height
        uint256 keyBlockHash = GetRandomXKeyBlockHash(height, pindex, params);
        if (keyBlockHash.IsNull()) {
            // Can't determine key block - reject
            return false;
        }

        uint256 randomxHash = CalculateRandomXHash(header, keyBlockHash);
        // Use height-aware CheckProofOfWorkImpl for RandomX with its own powLimit
        return CheckProofOfWorkImpl(randomxHash, header.nBits, height, params);
    } else {
        // SHA256d proof-of-work for legacy blocks
        return CheckProofOfWork(header.GetHash(), header.nBits, params);
    }
}

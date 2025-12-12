// Copyright (c) 2009-2010 Satoshi Nakamoto
// Copyright (c) 2009-2022 The Bitcoin Core developers
// Copyright (c) 2025-present The OpenSyria Core developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#ifndef OPENSYRIA_CONSENSUS_PARAMS_H
#define OPENSYRIA_CONSENSUS_PARAMS_H

#include <script/verify_flags.h>
#include <uint256.h>

#include <array>
#include <chrono>
#include <limits>
#include <map>
#include <vector>

namespace Consensus {

/**
 * A buried deployment is one where the height of the activation has been hardcoded into
 * the client implementation long after the consensus change has activated. See BIP 90.
 */
enum BuriedDeployment : int16_t {
    // buried deployments get negative values to avoid overlap with DeploymentPos
    DEPLOYMENT_HEIGHTINCB = std::numeric_limits<int16_t>::min(),
    DEPLOYMENT_CLTV,
    DEPLOYMENT_DERSIG,
    DEPLOYMENT_CSV,
    DEPLOYMENT_SEGWIT,
};
constexpr bool ValidDeployment(BuriedDeployment dep) { return dep <= DEPLOYMENT_SEGWIT; }

enum DeploymentPos : uint16_t {
    DEPLOYMENT_TESTDUMMY,
    DEPLOYMENT_TAPROOT, // Deployment of Schnorr/Taproot (BIPs 340-342)
    // NOTE: Also add new deployments to VersionBitsDeploymentInfo in deploymentinfo.cpp
    MAX_VERSION_BITS_DEPLOYMENTS
};
constexpr bool ValidDeployment(DeploymentPos dep) { return dep < MAX_VERSION_BITS_DEPLOYMENTS; }

/**
 * Struct for each individual consensus rule change using BIP9.
 */
struct BIP9Deployment {
    /** Bit position to select the particular bit in nVersion. */
    int bit{28};
    /** Start MedianTime for version bits miner confirmation. Can be a date in the past */
    int64_t nStartTime{NEVER_ACTIVE};
    /** Timeout/expiry MedianTime for the deployment attempt. */
    int64_t nTimeout{NEVER_ACTIVE};
    /** If lock in occurs, delay activation until at least this block
     *  height.  Note that activation will only occur on a retarget
     *  boundary.
     */
    int min_activation_height{0};
    /** Period of blocks to check signalling in (usually retarget period, ie params.DifficultyAdjustmentInterval()) */
    uint32_t period{2016};
    /**
     * Minimum blocks including miner confirmation of the total of 2016 blocks in a retargeting period,
     * which is also used for BIP9 deployments.
     * Examples: 1916 for 95%, 1512 for testchains.
     */
    uint32_t threshold{1916};

    /** Constant for nTimeout very far in the future. */
    static constexpr int64_t NO_TIMEOUT = std::numeric_limits<int64_t>::max();

    /** Special value for nStartTime indicating that the deployment is always active.
     *  This is useful for testing, as it means tests don't need to deal with the activation
     *  process (which takes at least 3 BIP9 intervals). Only tests that specifically test the
     *  behaviour during activation cannot use this. */
    static constexpr int64_t ALWAYS_ACTIVE = -1;

    /** Special value for nStartTime indicating that the deployment is never active.
     *  This is useful for integrating the code changes for a new feature
     *  prior to deploying it on some or all networks. */
    static constexpr int64_t NEVER_ACTIVE = -2;
};

/**
 * Parameters that influence chain consensus.
 */
struct Params {
    uint256 hashGenesisBlock;
    int nSubsidyHalvingInterval;
    /**
     * Hashes of blocks that
     * - are known to be consensus valid, and
     * - buried in the chain, and
     * - fail if the default script verify flags are applied.
     */
    std::map<uint256, script_verify_flags> script_flag_exceptions;
    /** Block height and hash at which BIP34 becomes active */
    int BIP34Height;
    uint256 BIP34Hash;
    /** Block height at which BIP65 becomes active */
    int BIP65Height;
    /** Block height at which BIP66 becomes active */
    int BIP66Height;
    /** Block height at which CSV (BIP68, BIP112 and BIP113) becomes active */
    int CSVHeight;
    /** Block height at which Segwit (BIP141, BIP143 and BIP147) becomes active.
     * Note that segwit v0 script rules are enforced on all blocks except the
     * BIP 16 exception blocks. */
    int SegwitHeight;
    /** Don't warn about unknown BIP 9 activations below this height.
     * This prevents us from warning about the CSV and segwit activations. */
    int MinBIP9WarningHeight;
    std::array<BIP9Deployment,MAX_VERSION_BITS_DEPLOYMENTS> vDeployments;
    /** Proof of work parameters */
    uint256 powLimit;
    bool fPowAllowMinDifficultyBlocks;
    /**
      * Enforce BIP94 timewarp attack mitigation. On testnet4 this also enforces
      * the block storm mitigation.
      */
    bool enforce_BIP94;
    bool fPowNoRetargeting;
    int64_t nPowTargetSpacing;
    int64_t nPowTargetTimespan;
    std::chrono::seconds PowTargetSpacing() const
    {
        return std::chrono::seconds{nPowTargetSpacing};
    }
    int64_t DifficultyAdjustmentInterval() const { return nPowTargetTimespan / nPowTargetSpacing; }
    /** The best chain should have at least this much work */
    uint256 nMinimumChainWork;
    /** By default assume that the signatures in ancestors of this block are valid */
    uint256 defaultAssumeValid;

    /**
     * If true, witness commitments contain a payload equal to an OpenSyria Script solution
     * to the signet challenge. See BIP325.
     */
    bool signet_blocks{false};
    std::vector<uint8_t> signet_challenge;

    /**
     * RandomX Hard Fork Parameters
     *
     * OpenSyria switches from SHA256d to RandomX proof-of-work at nRandomXForkHeight
     * to democratize mining and prevent ASIC/GPU domination.
     */
    int nRandomXForkHeight{57500};        //!< Block height at which RandomX activates
    int nRandomXKeyBlockInterval{32};     //!< How often the RandomX key changes (blocks) - 32 for tighter security
    uint256 powLimitRandomX;              //!< Minimum difficulty for RandomX blocks (resets at fork)

    /** Check if RandomX proof-of-work is active at the given height */
    bool IsRandomXActive(int height) const
    {
        return height >= nRandomXForkHeight;
    }

    /** Get the appropriate powLimit based on block height (SHA256d vs RandomX) */
    const uint256& GetRandomXPowLimit(int height) const
    {
        if (IsRandomXActive(height) && !powLimitRandomX.IsNull()) {
            return powLimitRandomX;
        }
        return powLimit;
    }

    /** Get the key block height for RandomX at a given block height.
     *  The key is derived from a block nRandomXKeyBlockInterval blocks before the current key interval.
     *  @param height The block height to calculate key block for
     *  @return Height of the block whose hash is used as RandomX key
     *
     *  SECURITY NOTE: For heights in the first two key intervals (0 to 2*interval-1),
     *  this returns heights that may result in using genesis block as key. This is
     *  acceptable for bootstrap but means early blocks share the same RandomX key.
     *  Key rotation begins properly once height >= 2*nRandomXKeyBlockInterval.
     */
    int GetRandomXKeyBlockHeight(int height) const
    {
        // Key changes every nRandomXKeyBlockInterval blocks
        // Key for height H is block at: (H / interval) * interval - interval
        //
        // Examples with interval=32:
        //   height 0-31:  keyHeight = 0 - 32 = -32 -> clamped to 0 (genesis)
        //   height 32-63: keyHeight = 32 - 32 = 0 (genesis)
        //   height 64-95: keyHeight = 64 - 32 = 32
        //   height 96-127: keyHeight = 96 - 32 = 64
        //
        // This means blocks 1-63 all use genesis as their key block.
        // This is a known bootstrap trade-off documented for auditors.
        int keyHeight = (height / nRandomXKeyBlockInterval) * nRandomXKeyBlockInterval - nRandomXKeyBlockInterval;
        return keyHeight >= 0 ? keyHeight : 0;
    }

    int DeploymentHeight(BuriedDeployment dep) const
    {
        switch (dep) {
        case DEPLOYMENT_HEIGHTINCB:
            return BIP34Height;
        case DEPLOYMENT_CLTV:
            return BIP65Height;
        case DEPLOYMENT_DERSIG:
            return BIP66Height;
        case DEPLOYMENT_CSV:
            return CSVHeight;
        case DEPLOYMENT_SEGWIT:
            return SegwitHeight;
        } // no default case, so the compiler can warn about missing cases
        return std::numeric_limits<int>::max();
    }
};

} // namespace Consensus

#endif // OPENSYRIA_CONSENSUS_PARAMS_H

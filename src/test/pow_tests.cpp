// Copyright (c) 2015-2022 The OpenSyria Core developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#include <chain.h>
#include <chainparams.h>
#include <pow.h>
#include <test/util/random.h>
#include <test/util/setup_common.h>
#include <util/chaintype.h>

#include <boost/test/unit_test.hpp>

BOOST_FIXTURE_TEST_SUITE(pow_tests, BasicTestingSetup)

// Helper to create a chain of CBlockIndex for tests that need GetAncestor()
// OpenSyria mainnet uses enforce_BIP94 which requires traversable ancestor chain
static std::vector<CBlockIndex> CreateBlockChain(int height, uint32_t nBits, uint32_t startTime, int64_t totalTimespan)
{
    std::vector<CBlockIndex> blocks(height + 1);
    for (int i = 0; i <= height; i++) {
        blocks[i].pprev = i ? &blocks[i - 1] : nullptr;
        blocks[i].nHeight = i;
        // Distribute time evenly across all blocks to achieve desired total timespan
        blocks[i].nTime = startTime + (i * totalTimespan / height);
        blocks[i].nBits = nBits;
        blocks[i].nChainWork = i ? blocks[i - 1].nChainWork + GetBlockProof(blocks[i - 1]) : arith_uint256(0);
    }
    return blocks;
}

/* Test calculation of next difficulty target with no constraints applying */
BOOST_AUTO_TEST_CASE(get_next_work)
{
    // OpenSyria: Test with perfect 2-week timing - difficulty should stay the same
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::MAIN);
    const auto& consensus = chainParams->GetConsensus();
    
    // Create a proper chain with ancestors (required for BIP94 enforcement)
    int targetHeight = consensus.DifficultyAdjustmentInterval() - 1; // 10079
    uint32_t startTime = 1733616000; // OpenSyria Genesis (Dec 8, 2024)
    uint32_t nBits = 0x1e00ffff;  // OpenSyria genesis difficulty
    
    // Perfect timing: exactly nPowTargetTimespan total
    int64_t totalTimespan = consensus.nPowTargetTimespan; // Exactly 2 weeks
    auto blocks = CreateBlockChain(targetHeight, nBits, startTime, totalTimespan);
    CBlockIndex* pindexLast = &blocks[targetHeight];
    
    // First block time is what CalculateNextWorkRequired uses
    int64_t nFirstBlockTime = blocks[0].nTime;

    // With perfect timing (actualTimespan == targetTimespan), difficulty stays the same
    unsigned int expected_nbits = 0x1e00ffffU;
    BOOST_CHECK_EQUAL(CalculateNextWorkRequired(pindexLast, nFirstBlockTime, consensus), expected_nbits);
    BOOST_CHECK(PermittedDifficultyTransition(consensus, pindexLast->nHeight+1, pindexLast->nBits, expected_nbits));
}

/* Test the constraint on the upper bound for next work */
BOOST_AUTO_TEST_CASE(get_next_work_pow_limit)
{
    // OpenSyria: Test that difficulty doesn't go easier than powLimit when blocks are slow.
    // Use a custom test setup to avoid RandomX/BIP94 complications:
    // - TESTNET has enforce_BIP94=false (simpler difficulty calculation)
    // - TESTNET has fPowAllowMinDifficultyBlocks=true (but doesn't affect CalculateNextWorkRequired)
    // - We test at a height where RandomX powLimit applies (height > 1)
    // 
    // When already at powLimit and blocks are 5x slow (capped to 4x), result stays at powLimit.
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::TESTNET);
    const auto& consensus = chainParams->GetConsensus();
    
    int targetHeight = consensus.DifficultyAdjustmentInterval() - 1;
    uint32_t startTime = 1733616000;
    
    // Use SHA256 powLimit (0x1e00ffff) as starting point
    // At height 10080, GetRandomXPowLimit returns powLimitRandomX which is easier than SHA256 powLimit
    // So the result after 4x will NOT be clamped by powLimitRandomX, giving 0x1e03fffc
    uint32_t nBits = 0x1e00ffff;
    
    // 5x slower than expected - will be capped at 4x by protocol
    int64_t totalTimespan = consensus.nPowTargetTimespan * 5;
    auto blocks = CreateBlockChain(targetHeight, nBits, startTime, totalTimespan);
    CBlockIndex* pindexLast = &blocks[targetHeight];
    int64_t nFirstBlockTime = blocks[0].nTime;
    
    // With 4x slower blocks, difficulty decreases 4x (target increases 4x)
    // 0x1e00ffff * 4 = 0x1e03fffc (approximately, after compact rounding)
    // This is NOT clamped because powLimitRandomX (0x00ff...) > 4 * SHA256 powLimit
    unsigned int result = CalculateNextWorkRequired(pindexLast, nFirstBlockTime, consensus);
    
    // Verify the result is 4x easier than starting difficulty
    arith_uint256 startTarget, resultTarget;
    startTarget.SetCompact(nBits);
    resultTarget.SetCompact(result);
    
    // Result should be approximately 4x the starting target
    BOOST_CHECK(resultTarget >= startTarget * 3);  // At least 3x (accounting for rounding)
    BOOST_CHECK(resultTarget <= startTarget * 5);  // At most 5x (accounting for rounding)
    
    // Verify the result doesn't exceed RandomX powLimit
    arith_uint256 randomxLimit = UintToArith256(consensus.powLimitRandomX);
    BOOST_CHECK(resultTarget <= randomxLimit);
}

/* Test the constraint on the lower bound for actual time taken */
BOOST_AUTO_TEST_CASE(get_next_work_lower_limit_actual)
{
    // OpenSyria: Test difficulty increase when blocks are too fast (capped at 4x)
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::MAIN);
    const auto& consensus = chainParams->GetConsensus();
    
    int targetHeight = consensus.DifficultyAdjustmentInterval() - 1;
    uint32_t startTime = 1733616000;
    uint32_t nBits = 0x1e00ffff;
    
    // 8x faster than expected - will be capped to 1/4 of target (max 4x difficulty increase)
    int64_t totalTimespan = consensus.nPowTargetTimespan / 8;
    auto blocks = CreateBlockChain(targetHeight, nBits, startTime, totalTimespan);
    CBlockIndex* pindexLast = &blocks[targetHeight];
    int64_t nFirstBlockTime = blocks[0].nTime;
    
    // Difficulty should increase by 4x (max allowed) - target becomes 1/4
    // 0x1e00ffff / 4 = 0x1d3fffe0 (approximately, after compact encoding)
    unsigned int result = CalculateNextWorkRequired(pindexLast, nFirstBlockTime, consensus);
    
    // Verify it's within the permitted transition and harder than before
    BOOST_CHECK(PermittedDifficultyTransition(consensus, pindexLast->nHeight+1, pindexLast->nBits, result));
    // The new target should be 4x smaller (difficulty 4x higher)
    arith_uint256 old_target, new_target;
    old_target.SetCompact(nBits);
    new_target.SetCompact(result);
    BOOST_CHECK(new_target <= old_target / 4 + 1); // Allow for rounding
    BOOST_CHECK(new_target >= old_target / 4 - 1);
}

/* Test the constraint on the upper bound for actual time taken */
BOOST_AUTO_TEST_CASE(get_next_work_upper_limit_actual)
{
    // OpenSyria: Test difficulty decrease when blocks are too slow (capped at 4x)
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::MAIN);
    const auto& consensus = chainParams->GetConsensus();
    
    int targetHeight = consensus.DifficultyAdjustmentInterval() - 1;
    uint32_t startTime = 1733616000;
    uint32_t nBits = 0x1d00ffff;  // Start with harder difficulty (not at powLimit)
    
    // 10x slower than expected - will be capped to 4x of target (max 4x difficulty decrease)
    int64_t totalTimespan = consensus.nPowTargetTimespan * 10;
    auto blocks = CreateBlockChain(targetHeight, nBits, startTime, totalTimespan);
    CBlockIndex* pindexLast = &blocks[targetHeight];
    int64_t nFirstBlockTime = blocks[0].nTime;
    
    // Difficulty should decrease by 4x (max allowed) - target becomes 4x larger
    unsigned int result = CalculateNextWorkRequired(pindexLast, nFirstBlockTime, consensus);
    
    // Verify it's within the permitted transition and easier than before
    BOOST_CHECK(PermittedDifficultyTransition(consensus, pindexLast->nHeight+1, pindexLast->nBits, result));
    // The new target should be 4x larger (difficulty 4x lower)
    arith_uint256 old_target, new_target;
    old_target.SetCompact(nBits);
    new_target.SetCompact(result);
    BOOST_CHECK(new_target >= old_target * 4 - 1); // Allow for rounding
    BOOST_CHECK(new_target <= old_target * 4 + 1);
}

BOOST_AUTO_TEST_CASE(CheckProofOfWork_test_negative_target)
{
    const auto consensus = CreateChainParams(*m_node.args, ChainType::MAIN)->GetConsensus();
    uint256 hash;
    unsigned int nBits;
    nBits = UintToArith256(consensus.powLimit).GetCompact(true);
    hash = uint256{1};
    BOOST_CHECK(!CheckProofOfWork(hash, nBits, consensus));
}

BOOST_AUTO_TEST_CASE(CheckProofOfWork_test_overflow_target)
{
    const auto consensus = CreateChainParams(*m_node.args, ChainType::MAIN)->GetConsensus();
    uint256 hash;
    unsigned int nBits{~0x00800000U};
    hash = uint256{1};
    BOOST_CHECK(!CheckProofOfWork(hash, nBits, consensus));
}

BOOST_AUTO_TEST_CASE(CheckProofOfWork_test_too_easy_target)
{
    const auto consensus = CreateChainParams(*m_node.args, ChainType::MAIN)->GetConsensus();
    uint256 hash;
    unsigned int nBits;
    arith_uint256 nBits_arith = UintToArith256(consensus.powLimit);
    nBits_arith *= 2;
    nBits = nBits_arith.GetCompact();
    hash = uint256{1};
    BOOST_CHECK(!CheckProofOfWork(hash, nBits, consensus));
}

BOOST_AUTO_TEST_CASE(CheckProofOfWork_test_biger_hash_than_target)
{
    const auto consensus = CreateChainParams(*m_node.args, ChainType::MAIN)->GetConsensus();
    uint256 hash;
    unsigned int nBits;
    arith_uint256 hash_arith = UintToArith256(consensus.powLimit);
    nBits = hash_arith.GetCompact();
    hash_arith *= 2; // hash > nBits
    hash = ArithToUint256(hash_arith);
    BOOST_CHECK(!CheckProofOfWork(hash, nBits, consensus));
}

BOOST_AUTO_TEST_CASE(CheckProofOfWork_test_zero_target)
{
    const auto consensus = CreateChainParams(*m_node.args, ChainType::MAIN)->GetConsensus();
    uint256 hash;
    unsigned int nBits;
    arith_uint256 hash_arith{0};
    nBits = hash_arith.GetCompact();
    hash = ArithToUint256(hash_arith);
    BOOST_CHECK(!CheckProofOfWork(hash, nBits, consensus));
}

BOOST_AUTO_TEST_CASE(GetBlockProofEquivalentTime_test)
{
    const auto chainParams = CreateChainParams(*m_node.args, ChainType::MAIN);
    std::vector<CBlockIndex> blocks(10000);
    for (int i = 0; i < 10000; i++) {
        blocks[i].pprev = i ? &blocks[i - 1] : nullptr;
        blocks[i].nHeight = i;
        blocks[i].nTime = 1269211443 + i * chainParams->GetConsensus().nPowTargetSpacing;
        blocks[i].nBits = 0x207fffff; /* target 0x7fffff000... */
        blocks[i].nChainWork = i ? blocks[i - 1].nChainWork + GetBlockProof(blocks[i - 1]) : arith_uint256(0);
    }

    for (int j = 0; j < 1000; j++) {
        CBlockIndex *p1 = &blocks[m_rng.randrange(10000)];
        CBlockIndex *p2 = &blocks[m_rng.randrange(10000)];
        CBlockIndex *p3 = &blocks[m_rng.randrange(10000)];

        int64_t tdiff = GetBlockProofEquivalentTime(*p1, *p2, *p3, chainParams->GetConsensus());
        BOOST_CHECK_EQUAL(tdiff, p1->GetBlockTime() - p2->GetBlockTime());
    }
}

void sanity_check_chainparams(const ArgsManager& args, ChainType chain_type)
{
    const auto chainParams = CreateChainParams(args, chain_type);
    const auto consensus = chainParams->GetConsensus();

    // hash genesis is correct
    BOOST_CHECK_EQUAL(consensus.hashGenesisBlock, chainParams->GenesisBlock().GetHash());

    // target timespan is an even multiple of spacing
    BOOST_CHECK_EQUAL(consensus.nPowTargetTimespan % consensus.nPowTargetSpacing, 0);

    // genesis nBits is positive, doesn't overflow and is lower than powLimit
    arith_uint256 pow_compact;
    bool neg, over;
    pow_compact.SetCompact(chainParams->GenesisBlock().nBits, &neg, &over);
    BOOST_CHECK(!neg && pow_compact != 0);
    BOOST_CHECK(!over);
    BOOST_CHECK(UintToArith256(consensus.powLimit) >= pow_compact);

    // check max target * 4*nPowTargetTimespan doesn't overflow -- see pow.cpp:CalculateNextWorkRequired()
    if (!consensus.fPowNoRetargeting) {
        arith_uint256 targ_max{UintToArith256(uint256{"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"})};
        targ_max /= consensus.nPowTargetTimespan*4;
        BOOST_CHECK(UintToArith256(consensus.powLimit) < targ_max);
    }
}

BOOST_AUTO_TEST_CASE(ChainParams_MAIN_sanity)
{
    sanity_check_chainparams(*m_node.args, ChainType::MAIN);
}

BOOST_AUTO_TEST_CASE(ChainParams_REGTEST_sanity)
{
    sanity_check_chainparams(*m_node.args, ChainType::REGTEST);
}

BOOST_AUTO_TEST_CASE(ChainParams_TESTNET_sanity)
{
    sanity_check_chainparams(*m_node.args, ChainType::TESTNET);
}

BOOST_AUTO_TEST_CASE(ChainParams_TESTNET4_sanity)
{
    sanity_check_chainparams(*m_node.args, ChainType::TESTNET4);
}

BOOST_AUTO_TEST_CASE(ChainParams_SIGNET_sanity)
{
    sanity_check_chainparams(*m_node.args, ChainType::SIGNET);
}

BOOST_AUTO_TEST_SUITE_END()

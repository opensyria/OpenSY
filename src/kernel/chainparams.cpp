// Copyright (c) 2010 Qirsh Nakamoto
// Copyright (c) 2009-present The OpenSyria Core developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#include <kernel/chainparams.h>

#include <chainparamsseeds.h>
#include <consensus/amount.h>
#include <consensus/merkle.h>
#include <consensus/params.h>
#include <hash.h>
#include <kernel/messagestartchars.h>
#include <logging.h>
#include <primitives/block.h>
#include <primitives/transaction.h>
#include <script/interpreter.h>
#include <script/script.h>
#include <uint256.h>
#include <util/chaintype.h>
#include <util/strencodings.h>

#include <algorithm>
#include <cassert>
#include <cstdint>
#include <cstring>
#include <type_traits>

using namespace util::hex_literals;

// Workaround MSVC bug triggering C7595 when calling consteval constructors in
// initializer lists.
// https://developercommunity.visualstudio.com/t/Bogus-C7595-error-on-valid-C20-code/10906093
#if defined(_MSC_VER)
auto consteval_ctor(auto&& input) { return input; }
#else
#define consteval_ctor(input) (input)
#endif

static CBlock CreateGenesisBlock(const char* pszTimestamp, const CScript& genesisOutputScript, uint32_t nTime, uint32_t nNonce, uint32_t nBits, int32_t nVersion, const CAmount& genesisReward)
{
    CMutableTransaction txNew;
    txNew.version = 1;
    txNew.vin.resize(1);
    txNew.vout.resize(1);
    txNew.vin[0].scriptSig = CScript() << 486604799 << CScriptNum(4) << std::vector<unsigned char>((const unsigned char*)pszTimestamp, (const unsigned char*)pszTimestamp + strlen(pszTimestamp));
    txNew.vout[0].nValue = genesisReward;
    txNew.vout[0].scriptPubKey = genesisOutputScript;

    CBlock genesis;
    genesis.nTime    = nTime;
    genesis.nBits    = nBits;
    genesis.nNonce   = nNonce;
    genesis.nVersion = nVersion;
    genesis.vtx.push_back(MakeTransactionRef(std::move(txNew)));
    genesis.hashPrevBlock.SetNull();
    genesis.hashMerkleRoot = BlockMerkleRoot(genesis);
    return genesis;
}

/**
 * Build the genesis block. Note that the output of its generation
 * transaction cannot be spent since it did not originally exist in the
 * database.
 *
 * CBlock(hash=000000000019d6, ver=1, hashPrevBlock=00000000000000, hashMerkleRoot=4a5e1e, nTime=1231006505, nBits=1d00ffff, nNonce=2083236893, vtx=1)
 *   CTransaction(hash=4a5e1e, ver=1, vin.size=1, vout.size=1, nLockTime=0)
 *     CTxIn(COutPoint(000000, -1), coinbase 04ffff001d0104455468652054696d65732030332f4a616e2f32303039204368616e63656c6c6f72206f6e206272696e6b206f66207365636f6e64206261696c6f757420666f722062616e6b73)
 *     CTxOut(nValue=50.00000000, scriptPubKey=0x5F1DF16B2B704C8A578D0B)
 *   vMerkleTree: 4a5e1e
 */
static CBlock CreateGenesisBlock(uint32_t nTime, uint32_t nNonce, uint32_t nBits, int32_t nVersion, const CAmount& genesisReward)
{
    const char* pszTimestamp = "Dec 8 2024 - Syria Liberated from Assad / سوريا حرة";
    const CScript genesisOutputScript = CScript() << "04678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5f"_hex << OP_CHECKSIG;
    return CreateGenesisBlock(pszTimestamp, genesisOutputScript, nTime, nNonce, nBits, nVersion, genesisReward);
}

/**
 * Main network on which people trade goods and services.
 */
class CMainParams : public CChainParams {
public:
    CMainParams() {
        m_chain_type = ChainType::MAIN;
        consensus.signet_blocks = false;
        consensus.signet_challenge.clear();
        consensus.nSubsidyHalvingInterval = 1050000; // ~4 years with 2-min blocks
        // No script flag exceptions for new chain - OpenSyria starts fresh
        consensus.BIP34Height = 1; // Active from block 1
        consensus.BIP34Hash = uint256{};
        consensus.BIP65Height = 1; // Active from block 1
        consensus.BIP66Height = 1; // Active from block 1
        consensus.CSVHeight = 1; // Active from block 1
        consensus.SegwitHeight = 1; // Active from block 1
        consensus.MinBIP9WarningHeight = 0;
        consensus.powLimit = uint256{"000000ffff000000000000000000000000000000000000000000000000000000"}; // Matches 0x1e00ffff
        consensus.nPowTargetTimespan = 14 * 24 * 60 * 60; // two weeks
        consensus.nPowTargetSpacing = 2 * 60; // 2-minute blocks
        consensus.fPowAllowMinDifficultyBlocks = false;
        // BIP94 timewarp attack protection - enabled for OpenSyria mainnet
        // Prevents manipulation of difficulty via timestamp attacks on difficulty period boundaries
        consensus.enforce_BIP94 = true;
        consensus.fPowNoRetargeting = false;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].bit = 28;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nStartTime = Consensus::BIP9Deployment::NEVER_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].threshold = 1815; // 90%
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].period = 2016;

        // Deployment of Taproot (BIPs 340-342) - Always active for OpenSyria
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].bit = 2;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nStartTime = Consensus::BIP9Deployment::ALWAYS_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].threshold = 1815; // 90%
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].period = 2016;

        // New chain starts with no minimum work requirement - will be updated as chain grows
        // TODO [SECURITY - HIGH PRIORITY]: Update nMinimumChainWork after first difficulty period (~2 weeks)
        // This protects against low-hashrate sybil attacks. Calculate using:
        //   opensyria-cli getblockchaininfo | grep chainwork
        // Then hardcode the value here in the next release.
        // TODO [SECURITY - SHA256d MITIGATION]: Since OpenSyria shares SHA256d with Bitcoin,
        // the chain is vulnerable to hashrate attacks from Bitcoin miners. Update this value
        // frequently during the first year of operation to prevent chain forgery attacks.
        consensus.nMinimumChainWork = uint256{};
        // TODO [SECURITY]: Update defaultAssumeValid to a trusted block hash after chain matures
        // This enables signature validation skipping for faster sync while maintaining security.
        consensus.defaultAssumeValid = uint256{}; // New chain - no assumed valid block yet

        // RandomX hard fork height - switch from SHA256d to RandomX PoW
        // This height gives miners ~80 days to prepare (57500 blocks * 2 min = ~80 days)
        consensus.nRandomXForkHeight = 57500;

        /**
         * The message start string is designed to be unlikely to occur in normal data.
         * The characters are rarely used upper ASCII, not valid as UTF-8, and produce
         * a large 32-bit integer with any alignment.
         */
        pchMessageStart[0] = 0x53; // 'S'
        pchMessageStart[1] = 0x59; // 'Y'
        pchMessageStart[2] = 0x4c; // 'L'
        pchMessageStart[3] = 0x4d; // 'M' for mainnet
        nDefaultPort = 9633; // OpenSyria mainnet port (963 = Syria country code)
        nPruneAfterHeight = 100000;
        m_assumed_blockchain_size = 1; // New chain - minimal initial size
        m_assumed_chain_state_size = 1; // New chain - minimal initial size

        genesis = CreateGenesisBlock(1733616000, 171081, 0x1e00ffff, 1, 10000 * COIN); // Dec 8, 2024 - Syria Liberation
        consensus.hashGenesisBlock = genesis.GetHash();
        assert(consensus.hashGenesisBlock == uint256{"0000000727ee231c405685355f07629b06bfcb462cfa1ed7de868a6d9590ca8d"});
        assert(genesis.hashMerkleRoot == uint256{"56f65e913353861d32d297c6bc87bbe81242b764d18b8634d75c5a0159c8859e"});

        // DNS seed nodes - cleared until OpenSyria seed infrastructure is established
        // For initial network bootstrap, use -addnode or -connect to connect to known nodes
        // TODO [CRITICAL - PRE-LAUNCH]: Set up OpenSyria DNS seed infrastructure:
        //   - seed.opensyria.net (primary)
        //   - seed2.opensyria.net (secondary, different jurisdiction)
        //   - seed-tor.opensyria.net (Tor users)
        // Without DNS seeds, the network cannot bootstrap automatically!
        // See: https://github.com/sipa/bitcoin-seeder for reference implementation
        vSeeds.clear();
        // TODO [CRITICAL - PRE-LAUNCH]: Uncomment and populate when DNS seeds are ready:
        // vSeeds.emplace_back("seed.opensyria.net");
        // vSeeds.emplace_back("seed2.opensyria.net");

        base58Prefixes[PUBKEY_ADDRESS] = std::vector<unsigned char>(1,35); // Addresses start with 'F' (Freedom)
        base58Prefixes[SCRIPT_ADDRESS] = std::vector<unsigned char>(1,36); // Script addresses start with 'F'
        base58Prefixes[SECRET_KEY] =     std::vector<unsigned char>(1,128); // WIF keys - Bitcoin mainnet compatible for wallet interop
        // Extended key prefixes kept Bitcoin-compatible for HD wallet interoperability
        // Future: Consider unique prefixes (e.g., spub/sprv) once ecosystem matures
        base58Prefixes[EXT_PUBLIC_KEY] = {0x04, 0x88, 0xB2, 0x1E}; // xpub - Bitcoin mainnet compatible
        base58Prefixes[EXT_SECRET_KEY] = {0x04, 0x88, 0xAD, 0xE4}; // xprv - Bitcoin mainnet compatible

        bech32_hrp = "syl"; // OpenSyria mainnet SegWit

        // TODO [CRITICAL - PRE-LAUNCH]: Populate fixed seeds with IP addresses of reliable nodes
        // Run contrib/seeds/generate-seeds.py after deploying 10+ geographically distributed nodes
        // Fixed seeds are fallback when DNS seeds fail - critical for network resilience
        vFixedSeeds.clear(); // No fixed seeds until OpenSyria network nodes are established

        fDefaultConsistencyChecks = false;
        m_is_mockable_chain = false;

        // AssumeUTXO data - empty for new chain, will be populated as chain grows
        // TODO [MAINTENANCE]: Generate AssumeUTXO snapshots at milestone heights (e.g., 100000, 500000)
        // This enables fast sync for new nodes. Use: opensyria-cli dumptxoutset <path>
        m_assumeutxo_data = {};


        // Chain transaction data - initialized for genesis, will be updated as chain grows
        // TODO [MAINTENANCE]: Update chainTxData periodically with actual chain statistics
        // Use: opensyria-cli getchaintxstats to get current values
        // This improves sync time estimation for new nodes
        chainTxData = ChainTxData{
            .nTime    = 1733616000, // Genesis timestamp - Dec 8, 2024
            .tx_count = 1,
            .dTxRate  = 0.001, // Initial low rate for new chain
        };


        // Headers sync parameters - conservative values for new chain
        m_headers_sync_params = HeadersSyncParams{
            .commitment_period = 100,
            .redownload_buffer_size = 2500, // Appropriate for new chain
        };

    }
};

/**
 * Testnet (v3): public test network which is reset from time to time.
 */
class CTestNetParams : public CChainParams {
public:
    CTestNetParams() {
        m_chain_type = ChainType::TESTNET;
        consensus.signet_blocks = false;
        consensus.signet_challenge.clear();
        consensus.nSubsidyHalvingInterval = 1050000; // ~4 years with 2-min blocks
        // No script flag exceptions for new chain - OpenSyria starts fresh
        consensus.BIP34Height = 1; // Active from block 1
        consensus.BIP34Hash = uint256{};
        consensus.BIP65Height = 1; // Active from block 1
        consensus.BIP66Height = 1; // Active from block 1
        consensus.CSVHeight = 1; // Active from block 1
        consensus.SegwitHeight = 1; // Active from block 1
        consensus.MinBIP9WarningHeight = 0;
        consensus.powLimit = uint256{"000000ffff000000000000000000000000000000000000000000000000000000"}; // Matches 0x1e00ffff
        consensus.nPowTargetTimespan = 14 * 24 * 60 * 60; // two weeks
        consensus.nPowTargetSpacing = 2 * 60; // 2-minute blocks
        consensus.fPowAllowMinDifficultyBlocks = true;
        consensus.enforce_BIP94 = false;
        consensus.fPowNoRetargeting = false;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].bit = 28;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nStartTime = Consensus::BIP9Deployment::NEVER_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].threshold = 1512; // 75%
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].period = 2016;

        // Deployment of Taproot (BIPs 340-342) - Always active for OpenSyria testnet
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].bit = 2;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nStartTime = Consensus::BIP9Deployment::ALWAYS_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].threshold = 1512; // 75%
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].period = 2016;

        // New chain starts with no minimum work requirement
        consensus.nMinimumChainWork = uint256{};
        consensus.defaultAssumeValid = uint256{}; // New chain - no assumed valid block yet

        // RandomX hard fork height - same as mainnet for consistency
        consensus.nRandomXForkHeight = 57500;

        pchMessageStart[0] = 0x53; // 'S'
        pchMessageStart[1] = 0x59; // 'Y'
        pchMessageStart[2] = 0x4c; // 'L'
        pchMessageStart[3] = 0x54; // 'T' for testnet
        nDefaultPort = 19633; // OpenSyria testnet port (1 + 963)
        nPruneAfterHeight = 1000;
        m_assumed_blockchain_size = 1; // New chain - minimal initial size
        m_assumed_chain_state_size = 1; // New chain - minimal initial size

        genesis = CreateGenesisBlock(1733616001, 7249204, 0x1e00ffff, 1, 10000 * COIN); // Testnet - Syria Liberation +1s
        consensus.hashGenesisBlock = genesis.GetHash();
        assert(consensus.hashGenesisBlock == uint256{"000000889cc24ca50c0ed047c43932757c1b7a6af418e13a10589ef968d44926"});
        assert(genesis.hashMerkleRoot == uint256{"56f65e913353861d32d297c6bc87bbe81242b764d18b8634d75c5a0159c8859e"});

        vFixedSeeds.clear();
        vSeeds.clear();
        // DNS seeds cleared until OpenSyria testnet seed infrastructure is established
        // Use -addnode or -connect for initial bootstrap

        base58Prefixes[PUBKEY_ADDRESS] = std::vector<unsigned char>(1,95);  // Testnet addresses start with 'f' (freedom)
        base58Prefixes[SCRIPT_ADDRESS] = std::vector<unsigned char>(1,96);  // Script addresses start with 'f'
        base58Prefixes[SECRET_KEY] =     std::vector<unsigned char>(1,239); // WIF testnet keys - Bitcoin testnet compatible for test compatibility
        // Extended key prefixes kept Bitcoin testnet-compatible for test compatibility
        base58Prefixes[EXT_PUBLIC_KEY] = {0x04, 0x35, 0x87, 0xCF}; // tpub - Bitcoin testnet compatible
        base58Prefixes[EXT_SECRET_KEY] = {0x04, 0x35, 0x83, 0x94}; // tprv - Bitcoin testnet compatible

        bech32_hrp = "tsyl"; // OpenSyria testnet4 SegWit

        vFixedSeeds.clear(); // No fixed seeds until OpenSyria testnet nodes are established

        fDefaultConsistencyChecks = false;
        m_is_mockable_chain = false;

        // AssumeUTXO data - empty for new chain
        m_assumeutxo_data = {};


        // Chain transaction data - initialized for genesis
        chainTxData = ChainTxData{
            .nTime    = 1733616001, // Testnet genesis timestamp
            .tx_count = 1,
            .dTxRate  = 0.001, // Initial low rate for new chain
        };


        // Headers sync parameters - conservative values for new chain
        m_headers_sync_params = HeadersSyncParams{
            .commitment_period = 100,
            .redownload_buffer_size = 2500,
        };

    }
};

/**
 * Testnet (v4): public test network which is reset from time to time.
 */
class CTestNet4Params : public CChainParams {
public:
    CTestNet4Params() {
        m_chain_type = ChainType::TESTNET4;
        consensus.signet_blocks = false;
        consensus.signet_challenge.clear();
        consensus.nSubsidyHalvingInterval = 1050000; // ~4 years with 2-min blocks
        consensus.BIP34Height = 1;
        consensus.BIP34Hash = uint256{};
        consensus.BIP65Height = 1;
        consensus.BIP66Height = 1;
        consensus.CSVHeight = 1;
        consensus.SegwitHeight = 1;
        consensus.MinBIP9WarningHeight = 0;
        consensus.powLimit = uint256{"000000ffff000000000000000000000000000000000000000000000000000000"}; // Matches 0x1e00ffff
        consensus.nPowTargetTimespan = 14 * 24 * 60 * 60; // two weeks
        consensus.nPowTargetSpacing = 2 * 60; // 2-minute blocks
        consensus.fPowAllowMinDifficultyBlocks = true;
        consensus.enforce_BIP94 = true;
        consensus.fPowNoRetargeting = false;

        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].bit = 28;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nStartTime = Consensus::BIP9Deployment::NEVER_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].threshold = 1512; // 75%
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].period = 2016;

        // Deployment of Taproot (BIPs 340-342)
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].bit = 2;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nStartTime = Consensus::BIP9Deployment::ALWAYS_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].threshold = 1512; // 75%
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].period = 2016;

        // New chain starts with no minimum work requirement
        consensus.nMinimumChainWork = uint256{};
        consensus.defaultAssumeValid = uint256{}; // New chain - no assumed valid block yet

        // RandomX hard fork height - same as mainnet for consistency
        consensus.nRandomXForkHeight = 57500;

        pchMessageStart[0] = 0x53; // 'S'
        pchMessageStart[1] = 0x59; // 'Y'
        pchMessageStart[2] = 0x4c; // 'L'
        pchMessageStart[3] = 0x34; // '4' for testnet4
        nDefaultPort = 49633; // OpenSyria testnet4 port (4 + 963)
        nPruneAfterHeight = 1000;
        m_assumed_blockchain_size = 1; // New chain - minimal initial size
        m_assumed_chain_state_size = 1; // New chain - minimal initial size

        genesis = CreateGenesisBlock(1733616004, 2023493, 0x1e00ffff, 1, 10000 * COIN); // Testnet4 - Syria Liberation +4s
        consensus.hashGenesisBlock = genesis.GetHash();
        assert(consensus.hashGenesisBlock == uint256{"0000005be5c111d92ec23198e3f5aa3fdf0b42d760611b97c5383500dfdcad9a"});
        assert(genesis.hashMerkleRoot == uint256{"56f65e913353861d32d297c6bc87bbe81242b764d18b8634d75c5a0159c8859e"});

        vFixedSeeds.clear();
        vSeeds.clear();
        // DNS seeds cleared until OpenSyria testnet4 seed infrastructure is established
        // Use -addnode or -connect for initial bootstrap

        base58Prefixes[PUBKEY_ADDRESS] = std::vector<unsigned char>(1,95);  // Testnet addresses start with 'f' (freedom)
        base58Prefixes[SCRIPT_ADDRESS] = std::vector<unsigned char>(1,96);  // Script addresses start with 'f'
        base58Prefixes[SECRET_KEY] =     std::vector<unsigned char>(1,239); // WIF testnet keys - Bitcoin testnet compatible for test compatibility
        // Extended key prefixes kept Bitcoin testnet-compatible for test compatibility
        base58Prefixes[EXT_PUBLIC_KEY] = {0x04, 0x35, 0x87, 0xCF}; // tpub - Bitcoin testnet compatible
        base58Prefixes[EXT_SECRET_KEY] = {0x04, 0x35, 0x83, 0x94}; // tprv - Bitcoin testnet compatible

        bech32_hrp = "tsyl"; // OpenSyria testnet SegWit

        vFixedSeeds.clear(); // No fixed seeds until OpenSyria testnet4 nodes are established

        fDefaultConsistencyChecks = false;
        m_is_mockable_chain = false;

        // AssumeUTXO data - empty for new chain
        m_assumeutxo_data = {};


        // Chain transaction data - initialized for genesis
        chainTxData = ChainTxData{
            .nTime    = 1733616004, // Testnet4 genesis timestamp
            .tx_count = 1,
            .dTxRate  = 0.001, // Initial low rate for new chain
        };


        // Headers sync parameters - conservative values for new chain
        m_headers_sync_params = HeadersSyncParams{
            .commitment_period = 100,
            .redownload_buffer_size = 2500,
        };

    }
};

/**
 * Signet: test network with an additional consensus parameter (see BIP325).
 */
class SigNetParams : public CChainParams {
public:
    explicit SigNetParams(const SigNetOptions& options)
    {
        std::vector<uint8_t> bin;
        vFixedSeeds.clear();
        vSeeds.clear();

        if (!options.challenge) {
            // TODO: Generate OpenSyria-specific signet challenge keys
            // For now, using a placeholder - replace with actual OpenSyria signet keys before launch
            bin = "512103ad5e0edad18cb1f0fc0d28a3d4f1f3e445640337489abb10404f2d1e086be430210359ef5021964fe22d6f8e05b2463c9540ce96883fe3b278760f048f5189f2e6c452ae"_hex_v_u8;
            vFixedSeeds.clear(); // No fixed seeds until OpenSyria signet nodes are established
            vSeeds.clear(); // DNS seeds cleared until OpenSyria signet infrastructure is established

            // New chain starts with no minimum work requirement
            consensus.nMinimumChainWork = uint256{};
            consensus.defaultAssumeValid = uint256{}; // New chain - no assumed valid block yet
            m_assumed_blockchain_size = 1; // New chain - minimal initial size
            m_assumed_chain_state_size = 1; // New chain - minimal initial size
            chainTxData = ChainTxData{
                .nTime    = 1733616002, // Signet genesis timestamp
                .tx_count = 1,
                .dTxRate  = 0.001, // Initial low rate for new chain
            };

        } else {
            bin = *options.challenge;
            consensus.nMinimumChainWork = uint256{};
            consensus.defaultAssumeValid = uint256{};
            m_assumed_blockchain_size = 0;
            m_assumed_chain_state_size = 0;
            chainTxData = ChainTxData{
                0,
                0,
                0,
            };

            LogInfo("Signet with challenge %s", HexStr(bin));
        }

        if (options.seeds) {
            vSeeds = *options.seeds;
        }

        m_chain_type = ChainType::SIGNET;
        consensus.signet_blocks = true;
        consensus.signet_challenge.assign(bin.begin(), bin.end());
        consensus.nSubsidyHalvingInterval = 1050000; // ~4 years with 2-min blocks
        consensus.BIP34Height = 1;
        consensus.BIP34Hash = uint256{};
        consensus.BIP65Height = 1;
        consensus.BIP66Height = 1;
        consensus.CSVHeight = 1;
        consensus.SegwitHeight = 1;
        consensus.nPowTargetTimespan = 14 * 24 * 60 * 60; // two weeks
        consensus.nPowTargetSpacing = 2 * 60; // 2-minute blocks
        consensus.fPowAllowMinDifficultyBlocks = false;
        consensus.enforce_BIP94 = false;
        consensus.fPowNoRetargeting = false;
        consensus.MinBIP9WarningHeight = 0;
        consensus.powLimit = uint256{"00000377ae000000000000000000000000000000000000000000000000000000"};
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].bit = 28;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nStartTime = Consensus::BIP9Deployment::NEVER_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].threshold = 1815; // 90%
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].period = 2016;

        // Activation of Taproot (BIPs 340-342)
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].bit = 2;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nStartTime = Consensus::BIP9Deployment::ALWAYS_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].threshold = 1815; // 90%
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].period = 2016;

        // RandomX hard fork height - same as mainnet for consistency
        consensus.nRandomXForkHeight = 57500;

        // message start is defined as the first 4 bytes of the sha256d of the block script
        HashWriter h{};
        h << consensus.signet_challenge;
        uint256 hash = h.GetHash();
        std::copy_n(hash.begin(), 4, pchMessageStart.begin());

        nDefaultPort = 39633; // OpenSyria signet port (3 + 963)
        nPruneAfterHeight = 1000;

        genesis = CreateGenesisBlock(1733616002, 14059426, 0x1e0377ae, 1, 10000 * COIN); // Signet - Syria Liberation +2s
        consensus.hashGenesisBlock = genesis.GetHash();
        assert(consensus.hashGenesisBlock == uint256{"000002f2691d8ba8b470635c448adb1e618a874a910e8955ed5c46cd5bd3ca9f"});
        assert(genesis.hashMerkleRoot == uint256{"56f65e913353861d32d297c6bc87bbe81242b764d18b8634d75c5a0159c8859e"});

        // AssumeUTXO data - empty for new chain
        m_assumeutxo_data = {};


        base58Prefixes[PUBKEY_ADDRESS] = std::vector<unsigned char>(1,95);  // Signet addresses start with 'f' (freedom)
        base58Prefixes[SCRIPT_ADDRESS] = std::vector<unsigned char>(1,96);  // Script addresses start with 'f'
        base58Prefixes[SECRET_KEY] =     std::vector<unsigned char>(1,239); // WIF testnet keys - Bitcoin testnet compatible for test compatibility
        // Extended key prefixes kept Bitcoin testnet-compatible for test compatibility
        base58Prefixes[EXT_PUBLIC_KEY] = {0x04, 0x35, 0x87, 0xCF}; // tpub - Bitcoin testnet compatible
        base58Prefixes[EXT_SECRET_KEY] = {0x04, 0x35, 0x83, 0x94}; // tprv - Bitcoin testnet compatible

        bech32_hrp = "tsyl"; // OpenSyria signet SegWit

        fDefaultConsistencyChecks = false;
        m_is_mockable_chain = false;

        // Headers sync parameters - conservative values for new chain
        m_headers_sync_params = HeadersSyncParams{
            .commitment_period = 100,
            .redownload_buffer_size = 2500,
        };

    }
};

/**
 * Regression test: intended for private networks only. Has minimal difficulty to ensure that
 * blocks can be found instantly.
 */
class CRegTestParams : public CChainParams
{
public:
    explicit CRegTestParams(const RegTestOptions& opts)
    {
        m_chain_type = ChainType::REGTEST;
        consensus.signet_blocks = false;
        consensus.signet_challenge.clear();
        consensus.nSubsidyHalvingInterval = 150;
        consensus.BIP34Height = 1; // Always active unless overridden
        consensus.BIP34Hash = uint256();
        consensus.BIP65Height = 1;  // Always active unless overridden
        consensus.BIP66Height = 1;  // Always active unless overridden
        consensus.CSVHeight = 1;    // Always active unless overridden
        consensus.SegwitHeight = 0; // Always active unless overridden
        consensus.MinBIP9WarningHeight = 0;
        consensus.powLimit = uint256{"7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"};
        consensus.nPowTargetTimespan = 24 * 60 * 60; // one day
        consensus.nPowTargetSpacing = 2 * 60; // 2-minute blocks
        consensus.fPowAllowMinDifficultyBlocks = true;
        consensus.enforce_BIP94 = opts.enforce_bip94;
        consensus.fPowNoRetargeting = true;

        // RandomX fork at low height for regtest testing
        consensus.nRandomXForkHeight = 200;

        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].bit = 28;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nStartTime = 0;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].threshold = 108; // 75%
        consensus.vDeployments[Consensus::DEPLOYMENT_TESTDUMMY].period = 144; // Faster than normal for regtest (144 instead of 2016)

        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].bit = 2;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nStartTime = Consensus::BIP9Deployment::ALWAYS_ACTIVE;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].nTimeout = Consensus::BIP9Deployment::NO_TIMEOUT;
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].min_activation_height = 0; // No activation delay
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].threshold = 108; // 75%
        consensus.vDeployments[Consensus::DEPLOYMENT_TAPROOT].period = 144;

        consensus.nMinimumChainWork = uint256{};
        consensus.defaultAssumeValid = uint256{};

        pchMessageStart[0] = 0x53; // 'S'
        pchMessageStart[1] = 0x59; // 'Y'
        pchMessageStart[2] = 0x4c; // 'L'
        pchMessageStart[3] = 0x52; // 'R' for regtest
        nDefaultPort = 19634; // OpenSyria regtest port (1 + 963 + 4)
        nPruneAfterHeight = opts.fastprune ? 100 : 1000;
        m_assumed_blockchain_size = 0;
        m_assumed_chain_state_size = 0;

        for (const auto& [dep, height] : opts.activation_heights) {
            switch (dep) {
            case Consensus::BuriedDeployment::DEPLOYMENT_SEGWIT:
                consensus.SegwitHeight = int{height};
                break;
            case Consensus::BuriedDeployment::DEPLOYMENT_HEIGHTINCB:
                consensus.BIP34Height = int{height};
                break;
            case Consensus::BuriedDeployment::DEPLOYMENT_DERSIG:
                consensus.BIP66Height = int{height};
                break;
            case Consensus::BuriedDeployment::DEPLOYMENT_CLTV:
                consensus.BIP65Height = int{height};
                break;
            case Consensus::BuriedDeployment::DEPLOYMENT_CSV:
                consensus.CSVHeight = int{height};
                break;
            }
        }

        for (const auto& [deployment_pos, version_bits_params] : opts.version_bits_parameters) {
            consensus.vDeployments[deployment_pos].nStartTime = version_bits_params.start_time;
            consensus.vDeployments[deployment_pos].nTimeout = version_bits_params.timeout;
            consensus.vDeployments[deployment_pos].min_activation_height = version_bits_params.min_activation_height;
        }

        genesis = CreateGenesisBlock(1733616003, 2, 0x207fffff, 1, 10000 * COIN); // Regtest - Syria Liberation +3s
        consensus.hashGenesisBlock = genesis.GetHash();
        assert(consensus.hashGenesisBlock == uint256{"67fb155259a269da63429b2d84149027fc4a9a366236bc849fddff3a2554cd50"});
        assert(genesis.hashMerkleRoot == uint256{"56f65e913353861d32d297c6bc87bbe81242b764d18b8634d75c5a0159c8859e"});

        vFixedSeeds.clear(); //!< Regtest mode doesn't have any fixed seeds.
        vSeeds.clear();
        vSeeds.emplace_back("dummySeed.invalid.");

        fDefaultConsistencyChecks = true;
        m_is_mockable_chain = true;

        // AssumeUTXO data for OpenSyria regtest
        // Generated using test framework's deterministic block generation
        m_assumeutxo_data = {
            {
                // For use by unit tests
                .height = 110,
                .hash_serialized = AssumeutxoHash{uint256{"307d034c22a1d1f7d21e26bbe005ddbd01c28664a6c808d1499249a52e0c535a"}},
                .m_chain_tx_count = 111,
                .blockhash = uint256{"5d6cb6d0b8ad7441634b617315d0dd51a8f63d3b8122981489bedda7ac9cac61"},
            },
            {
                // For use by test/functional/feature_assumeutxo.py
                .height = 299,
                .hash_serialized = AssumeutxoHash{uint256{"e2c222db5361eb6ae9cd3f36e1addb32514eb59e2a8cdc4d3cd1489b4fcb11e3"}},
                .m_chain_tx_count = 334,
                .blockhash = uint256{"247f58c5696ad5e062a29ab74269a495aa25031bb1a359edd5969c3edcb02921"},
            },
        };

        chainTxData = ChainTxData{
            .nTime = 0,
            .tx_count = 0,
            .dTxRate = 0.001, // Set a non-zero rate to make it testable
        };


        base58Prefixes[PUBKEY_ADDRESS] = std::vector<unsigned char>(1,95);  // Regtest addresses start with 'f' (freedom)
        base58Prefixes[SCRIPT_ADDRESS] = std::vector<unsigned char>(1,96);  // Script addresses start with 'f'
        base58Prefixes[SECRET_KEY] =     std::vector<unsigned char>(1,239); // WIF testnet keys - Bitcoin testnet compatible for test compatibility
        // Extended key prefixes kept Bitcoin testnet-compatible for test compatibility
        base58Prefixes[EXT_PUBLIC_KEY] = {0x04, 0x35, 0x87, 0xCF}; // tpub - Bitcoin testnet compatible
        base58Prefixes[EXT_SECRET_KEY] = {0x04, 0x35, 0x83, 0x94}; // tprv - Bitcoin testnet compatible

        bech32_hrp = "rsyl"; // OpenSyria regtest SegWit

        // Copied from Testnet4.
        m_headers_sync_params = HeadersSyncParams{
            .commitment_period = 275,
            .redownload_buffer_size = 7017, // 7017/275 = ~25.5 commitments
        };

    }
};

std::unique_ptr<const CChainParams> CChainParams::SigNet(const SigNetOptions& options)
{
    return std::make_unique<const SigNetParams>(options);
}

std::unique_ptr<const CChainParams> CChainParams::RegTest(const RegTestOptions& options)
{
    return std::make_unique<const CRegTestParams>(options);
}

std::unique_ptr<const CChainParams> CChainParams::Main()
{
    return std::make_unique<const CMainParams>();
}

std::unique_ptr<const CChainParams> CChainParams::TestNet()
{
    return std::make_unique<const CTestNetParams>();
}

std::unique_ptr<const CChainParams> CChainParams::TestNet4()
{
    return std::make_unique<const CTestNet4Params>();
}

std::vector<int> CChainParams::GetAvailableSnapshotHeights() const
{
    std::vector<int> heights;
    heights.reserve(m_assumeutxo_data.size());

    for (const auto& data : m_assumeutxo_data) {
        heights.emplace_back(data.height);
    }
    return heights;
}

std::optional<ChainType> GetNetworkForMagic(const MessageStartChars& message)
{
    const auto mainnet_msg = CChainParams::Main()->MessageStart();
    const auto testnet_msg = CChainParams::TestNet()->MessageStart();
    const auto testnet4_msg = CChainParams::TestNet4()->MessageStart();
    const auto regtest_msg = CChainParams::RegTest({})->MessageStart();
    const auto signet_msg = CChainParams::SigNet({})->MessageStart();

    if (std::ranges::equal(message, mainnet_msg)) {
        return ChainType::MAIN;
    } else if (std::ranges::equal(message, testnet_msg)) {
        return ChainType::TESTNET;
    } else if (std::ranges::equal(message, testnet4_msg)) {
        return ChainType::TESTNET4;
    } else if (std::ranges::equal(message, regtest_msg)) {
        return ChainType::REGTEST;
    } else if (std::ranges::equal(message, signet_msg)) {
        return ChainType::SIGNET;
    }
    return std::nullopt;
}

#include <kernel/chainparams.h>
#include <iostream>
#include <memory>

int main() {
    // Main
    auto mainParams = CChainParams::Main();
    std::cout << "MAINNET:\n";
    std::cout << "  hashGenesisBlock: " << mainParams->GenesisBlock().GetHash().ToString() << "\n";
    std::cout << "  hashMerkleRoot: " << mainParams->GenesisBlock().hashMerkleRoot.ToString() << "\n\n";

    // Testnet
    auto testParams = CChainParams::TestNet();
    std::cout << "TESTNET:\n";
    std::cout << "  hashGenesisBlock: " << testParams->GenesisBlock().GetHash().ToString() << "\n";
    std::cout << "  hashMerkleRoot: " << testParams->GenesisBlock().hashMerkleRoot.ToString() << "\n\n";

    // Testnet4
    auto test4Params = CChainParams::TestNet4();
    std::cout << "TESTNET4:\n";
    std::cout << "  hashGenesisBlock: " << test4Params->GenesisBlock().GetHash().ToString() << "\n";
    std::cout << "  hashMerkleRoot: " << test4Params->GenesisBlock().hashMerkleRoot.ToString() << "\n\n";

    // Signet
    auto signetParams = CChainParams::SigNet({});
    std::cout << "SIGNET:\n";
    std::cout << "  hashGenesisBlock: " << signetParams->GenesisBlock().GetHash().ToString() << "\n";
    std::cout << "  hashMerkleRoot: " << signetParams->GenesisBlock().hashMerkleRoot.ToString() << "\n\n";

    // Regtest
    auto regParams = CChainParams::RegTest({});
    std::cout << "REGTEST:\n";
    std::cout << "  hashGenesisBlock: " << regParams->GenesisBlock().GetHash().ToString() << "\n";
    std::cout << "  hashMerkleRoot: " << regParams->GenesisBlock().hashMerkleRoot.ToString() << "\n";

    return 0;
}

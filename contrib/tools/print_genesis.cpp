/**
 * OpenSyria Genesis Block Hash Printer
 * =====================================
 * 
 * PURPOSE:
 *   This utility prints the genesis block hashes for all OpenSyria networks.
 *   Useful for verifying genesis block configuration after chain parameter changes.
 * 
 * USAGE:
 *   This file requires linking against the OpenSyria libraries.
 *   It's meant to be compiled as part of the build system or manually with:
 *   
 *   From the build directory:
 *   $ g++ -std=c++20 -I../src -I../src/config print_genesis.cpp \
 *         -L./lib -lopensyria_common -lopensyria_util -lopensyria_crypto \
 *         -o print_genesis
 *   $ ./print_genesis
 * 
 * OUTPUT:
 *   Displays genesis block hash and merkle root for:
 *   - Mainnet
 *   - Testnet  
 *   - Testnet4
 *   - Signet
 *   - Regtest
 * 
 * WHEN TO USE:
 *   - After modifying genesis block parameters in chainparams.cpp
 *   - To verify genesis hashes match expected values
 *   - When setting up a new network
 * 
 * Copyright (c) 2025 The OpenSyria Core developers
 * Distributed under the MIT software license.
 */

#include <kernel/chainparams.h>
#include <iostream>
#include <memory>

int main() {
    std::cout << "╔═══════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║           OpenSyria Genesis Block Hashes                      ║" << std::endl;
    std::cout << "╚═══════════════════════════════════════════════════════════════╝" << std::endl;
    std::cout << std::endl;

    // Mainnet genesis
    auto mainParams = CChainParams::Main();
    std::cout << "MAINNET:" << std::endl;
    std::cout << "  Genesis Hash:  " << mainParams->GenesisBlock().GetHash().ToString() << std::endl;
    std::cout << "  Merkle Root:   " << mainParams->GenesisBlock().hashMerkleRoot.ToString() << std::endl;
    std::cout << std::endl;

    // Testnet genesis
    auto testParams = CChainParams::TestNet();
    std::cout << "TESTNET:" << std::endl;
    std::cout << "  Genesis Hash:  " << testParams->GenesisBlock().GetHash().ToString() << std::endl;
    std::cout << "  Merkle Root:   " << testParams->GenesisBlock().hashMerkleRoot.ToString() << std::endl;
    std::cout << std::endl;

    // Testnet4 genesis
    auto test4Params = CChainParams::TestNet4();
    std::cout << "TESTNET4:" << std::endl;
    std::cout << "  Genesis Hash:  " << test4Params->GenesisBlock().GetHash().ToString() << std::endl;
    std::cout << "  Merkle Root:   " << test4Params->GenesisBlock().hashMerkleRoot.ToString() << std::endl;
    std::cout << std::endl;

    // Signet genesis
    auto signetParams = CChainParams::SigNet({});
    std::cout << "SIGNET:" << std::endl;
    std::cout << "  Genesis Hash:  " << signetParams->GenesisBlock().GetHash().ToString() << std::endl;
    std::cout << "  Merkle Root:   " << signetParams->GenesisBlock().hashMerkleRoot.ToString() << std::endl;
    std::cout << std::endl;

    // Regtest genesis
    auto regParams = CChainParams::RegTest({});
    std::cout << "REGTEST:" << std::endl;
    std::cout << "  Genesis Hash:  " << regParams->GenesisBlock().GetHash().ToString() << std::endl;
    std::cout << "  Merkle Root:   " << regParams->GenesisBlock().hashMerkleRoot.ToString() << std::endl;
    std::cout << std::endl;

    std::cout << "NOTE: These hashes must match the values in src/kernel/chainparams.cpp" << std::endl;

    return 0;
}

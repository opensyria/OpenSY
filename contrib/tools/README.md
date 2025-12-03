# OpenSyria Development Tools

This directory contains standalone utilities created during the OpenSyria fork process.

## Files

### `genesis_miner.cpp`
Displays OpenSyria chain parameters and genesis block information.
- **Usage**: Standalone utility, compile with `g++ -o genesis_miner genesis_miner.cpp`
- **Purpose**: Reference for chain parameters when setting up new networks

### `print_genesis.cpp`  
Prints genesis block hashes for all OpenSyria networks (mainnet, testnet, regtest, etc.)
- **Usage**: Requires OpenSyria libraries, compile from build directory
- **Purpose**: Verify genesis block configuration after changes to chainparams.cpp

## OpenSyria Chain Parameters

| Parameter | Value |
|-----------|-------|
| Coin Symbol | SYL |
| Block Reward | 10,000 SYL |
| Max Supply | 21 billion SYL |
| Block Time | 2 minutes |
| Halving Interval | 210,000 blocks |
| Mainnet Port | 9633 |
| Testnet Port | 19633 |
| Address Prefix | 'S' (mainnet), 's' (testnet) |

## License

MIT License - see COPYING in the root directory.

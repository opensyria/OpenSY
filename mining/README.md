# OpenSY Mining

This directory contains mining-related scripts and tools for OpenSY.

## Directory Structure

```
mining/
├── scripts/          # Local mining shell scripts
│   ├── mine.sh       # Universal mining script (recommended)
│   └── mine_mainnet_v2.sh  # Alternative mainnet miner
└── vast-ai/          # Cloud mining on Vast.ai
    ├── Dockerfile
    ├── setup.sh
    └── start-mining.sh
```

## Quick Start (Local Mining)

```bash
# Using the universal mining script
./mining/scripts/mine.sh

# Or with a custom address
./mining/scripts/mine.sh syl1your_address_here
```

## Cloud Mining (Vast.ai)

See [vast-ai/README.md](vast-ai/README.md) for instructions on mining using cloud GPUs.

## Mining Algorithm

| Block Height | Algorithm | Hardware |
|--------------|-----------|----------|
| 0 (Genesis)  | SHA-256d  | N/A (pre-mined) |
| 1+           | RandomX   | CPU |

RandomX is ASIC-resistant and CPU-optimized, making mining accessible to everyone.

## Resources

- [Mining Hardware Benchmarks](../docs/mining/HARDWARE_BENCHMARKS.md)
- [Node Operator Guide](../docs/NODE_OPERATOR_GUIDE.md)

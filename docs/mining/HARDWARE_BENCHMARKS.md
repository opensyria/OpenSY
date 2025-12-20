# OpenSY Mining Hardware Benchmarks

**Version:** 1.0  
**Date:** December 20, 2025  
**Last Updated:** December 20, 2025

---

## Overview

OpenSY uses RandomX proof-of-work, which is optimized for general-purpose CPUs. This document provides benchmark data for various hardware configurations.

> **Community Contribution Welcome!** Submit your benchmark results via GitHub PR.

---

## How to Benchmark

### Quick Test (30 seconds)

```bash
# Build with release optimizations
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j$(nproc)

# Run built-in benchmark
./build/bin/bench_opensy -filter="RandomX.*"
```

### Extended Test (Mining)

```bash
# Time how long to mine 10 blocks
time ./build/bin/opensy-cli generatetoaddress 10 $(./build/bin/opensy-cli getnewaddress)
```

---

## CPU Benchmarks

### AMD CPUs

| CPU Model | Cores/Threads | H/s (Light) | H/s (Full) | Notes |
|-----------|---------------|-------------|------------|-------|
| EPYC 7742 | 64/128 | TBD | TBD | Server-class |
| EPYC 7551 | 32/64 | TBD | TBD | Server-class |
| Ryzen 9 7950X | 16/32 | TBD | TBD | Desktop flagship |
| Ryzen 9 5950X | 16/32 | TBD | TBD | Previous gen flagship |
| Ryzen 9 5900X | 12/24 | TBD | TBD | High-end desktop |
| Ryzen 7 7700X | 8/16 | TBD | TBD | Mid-range desktop |
| Ryzen 7 5800X | 8/16 | TBD | TBD | Previous mid-range |
| Ryzen 5 5600X | 6/12 | TBD | TBD | Budget desktop |
| Ryzen 5 3600 | 6/12 | TBD | TBD | Older budget |

### Intel CPUs

| CPU Model | Cores/Threads | H/s (Light) | H/s (Full) | Notes |
|-----------|---------------|-------------|------------|-------|
| Xeon Platinum 8380 | 40/80 | TBD | TBD | Server-class |
| Xeon Gold 6248 | 20/40 | TBD | TBD | Server-class |
| Core i9-14900K | 24/32 | TBD | TBD | Desktop flagship |
| Core i9-13900K | 24/32 | TBD | TBD | Previous flagship |
| Core i7-14700K | 20/28 | TBD | TBD | High-end desktop |
| Core i7-13700K | 16/24 | TBD | TBD | Previous high-end |
| Core i5-14600K | 14/20 | TBD | TBD | Mid-range |
| Core i5-13400 | 10/16 | TBD | TBD | Budget gaming |

### Apple Silicon

| CPU Model | Cores | H/s (Light) | H/s (Full) | Notes |
|-----------|-------|-------------|------------|-------|
| M3 Max | 16 CPU | TBD | TBD | Laptop flagship |
| M3 Pro | 12 CPU | TBD | TBD | Pro laptop |
| M3 | 8 CPU | TBD | TBD | Base M3 |
| M2 Ultra | 24 CPU | TBD | TBD | Desktop flagship |
| M2 Max | 12 CPU | TBD | TBD | Previous laptop |
| M2 | 8 CPU | TBD | TBD | Base M2 |
| M1 Ultra | 20 CPU | TBD | TBD | Previous desktop |
| M1 Max | 10 CPU | TBD | TBD | Previous laptop |
| M1 | 8 CPU | TBD | TBD | Original Apple Silicon |

### ARM Servers

| CPU Model | Cores | H/s (Light) | H/s (Full) | Notes |
|-----------|-------|-------------|------------|-------|
| AWS Graviton3 | 64 | TBD | TBD | Cloud instance |
| AWS Graviton2 | 64 | TBD | TBD | Previous gen |
| Ampere Altra | 80 | TBD | TBD | Cloud/server |

### Low-Power / Embedded

| Device | CPU | H/s (Light) | Notes |
|--------|-----|-------------|-------|
| Raspberry Pi 5 | BCM2712 | TBD | 4-core ARM |
| Raspberry Pi 4 | BCM2711 | TBD | Not recommended |
| Orange Pi 5 | RK3588S | TBD | 8-core ARM |

---

## Cloud Instance Benchmarks

### AWS EC2

| Instance Type | vCPUs | RAM | $/hour | H/s | $/kH |
|---------------|-------|-----|--------|-----|------|
| c7a.16xlarge | 64 | 128GB | ~$2.50 | TBD | TBD |
| c7a.8xlarge | 32 | 64GB | ~$1.25 | TBD | TBD |
| c7a.4xlarge | 16 | 32GB | ~$0.62 | TBD | TBD |
| c6a.4xlarge | 16 | 32GB | ~$0.61 | TBD | TBD |
| t3.2xlarge | 8 | 32GB | ~$0.33 | TBD | TBD |

### Google Cloud

| Instance Type | vCPUs | RAM | $/hour | H/s | $/kH |
|---------------|-------|-----|--------|-----|------|
| n2-highcpu-64 | 64 | 64GB | ~$2.00 | TBD | TBD |
| n2-highcpu-32 | 32 | 32GB | ~$1.00 | TBD | TBD |
| n2-standard-8 | 8 | 32GB | ~$0.40 | TBD | TBD |

### Digital Ocean

| Droplet | vCPUs | RAM | $/hour | H/s | $/kH |
|---------|-------|-----|--------|-----|------|
| CPU-Optimized 32 | 32 | 64GB | ~$0.95 | TBD | TBD |
| CPU-Optimized 16 | 16 | 32GB | ~$0.48 | TBD | TBD |
| CPU-Optimized 8 | 8 | 16GB | ~$0.24 | TBD | TBD |

---

## Memory Impact

RandomX has two modes:

| Mode | Memory Required | Performance | Use Case |
|------|-----------------|-------------|----------|
| **Light** | 256 MB | ~1x baseline | Validation |
| **Full** | 2 GB | ~4-6x faster | Mining |

Mining uses full mode automatically. Ensure you have 2GB+ free RAM for optimal performance.

---

## Optimal Settings

### Thread Count

```
Recommended threads = Physical cores - 1
```

This leaves one core for system tasks and network handling.

### NUMA Considerations

For multi-socket servers:
```bash
# Pin to single NUMA node for best performance
numactl --cpunodebind=0 --membind=0 ./build/bin/opensy-cli generatetoaddress ...
```

### CPU Governor

For maximum performance:
```bash
# Linux: Set performance governor
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
```

---

## Power Consumption

| CPU Class | Typical TDP | Mining Power | Cost/Day @ $0.10/kWh |
|-----------|-------------|--------------|----------------------|
| Server (64-core) | 225W | ~250W | ~$0.60 |
| Desktop (16-core) | 105W | ~150W | ~$0.36 |
| Desktop (8-core) | 65W | ~100W | ~$0.24 |
| Laptop (8-core) | 45W | ~60W | ~$0.14 |
| Apple M-series | 30W | ~40W | ~$0.10 |

---

## Submit Your Benchmark

To contribute benchmark data:

1. Fork the repository
2. Add your results to this file
3. Include:
   - Exact CPU model
   - RAM configuration
   - OS and version
   - OpenSY version
   - Benchmark methodology
4. Submit a Pull Request

### Template

```markdown
### [Your CPU Model]

| Metric | Value |
|--------|-------|
| CPU | [Model] |
| Cores/Threads | [X/Y] |
| RAM | [Amount] |
| OS | [OS Version] |
| OpenSY Version | [Version] |
| H/s (Light) | [Value] |
| H/s (Full) | [Value] |
| Power Draw | [Watts] |
| Tested By | [GitHub Username] |
| Date | [YYYY-MM-DD] |
```

---

## Notes

- **H/s** = Hashes per second
- **Light mode** = 256MB RAM, used for validation
- **Full mode** = 2GB RAM, used for mining
- RandomX performance scales nearly linearly with core count
- Cache size and memory bandwidth impact performance
- Results may vary based on system load, cooling, and configuration

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-20 | Initial benchmark template |

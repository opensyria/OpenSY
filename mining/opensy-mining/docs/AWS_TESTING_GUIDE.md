# OpenSY Mining - AWS Multi-Platform Testing Guide

This guide helps you test the RandomX CGO bindings across different AWS instance types.

## Recommended AWS Instances

| Platform | Instance Type | vCPUs | RAM | Architecture | Cost/hr |
|----------|---------------|-------|-----|--------------|---------|
| Linux x64 | t3.medium | 2 | 4GB | x86_64 | ~$0.04 |
| Linux x64 (perf) | c5.xlarge | 4 | 8GB | x86_64 | ~$0.17 |
| Linux ARM64 | t4g.medium | 2 | 4GB | ARM64 | ~$0.03 |
| Linux ARM64 (perf) | c6g.xlarge | 4 | 8GB | ARM64 | ~$0.14 |

**Note**: Full dataset tests require 4GB+ RAM. Use `*.large` or bigger for those tests.

## Quick Setup Script

Run this on a fresh Ubuntu 22.04 instance:

```bash
#!/bin/bash
# setup-instance.sh

set -e

echo "=== OpenSY Mining Test Environment Setup ==="

# Update system
sudo apt-get update
sudo apt-get install -y build-essential cmake git curl

# Install Go 1.21
GO_VERSION="1.21.5"
ARCH=$(dpkg --print-architecture)

if [ "$ARCH" = "amd64" ]; then
    GO_ARCH="amd64"
elif [ "$ARCH" = "arm64" ]; then
    GO_ARCH="arm64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

curl -LO "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
rm "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"

echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
export PATH=$PATH:/usr/local/go/bin

echo "Go version: $(go version)"

# Clone repository
git clone https://github.com/opensyria/OpenSY.git ~/OpenSY
cd ~/OpenSY/mining/opensy-mining

echo "=== Setup Complete ==="
echo "Run: cd ~/OpenSY/mining/opensy-mining && ./scripts/test-randomx.sh"
```

## Running Tests

### Quick Test (Unit tests only)
```bash
cd ~/OpenSY/mining/opensy-mining
./scripts/test-randomx.sh quick
```

### Full Test (Unit + Race + Dataset + Benchmarks)
```bash
./scripts/test-randomx.sh full
```

### Benchmarks Only
```bash
./scripts/test-randomx.sh bench
```

## Expected Results

### Test Vectors
The following hashes must match (from RandomX reference):

| Key | Input | Expected Hash |
|-----|-------|---------------|
| test key 000 | This is a test | 639183aae1bf4c9a35884cb46b09cad9175f04efd7684e7262a0ac1c2f0b4e3f |
| test key 000 | Lorem ipsum dolor sit amet | 300a0adb47603dedb42228ccb2b211104f4da45af709cd7547cd049e9489c969 |

### Performance Expectations

| Platform | Light Mode (H/s) | Full Dataset (H/s) |
|----------|------------------|-------------------|
| x64 (modern) | ~10-20 | ~100-300 |
| ARM64 (Graviton2) | ~8-15 | ~80-200 |
| x64 + AVX2 | ~15-30 | ~200-500 |

**Note**: Full dataset mode is ~10x faster but requires 2GB RAM.

## Platform-Specific Notes

### Linux x86_64
- Best compatibility, fastest performance
- Full JIT and AVX2 support
- Use `c5` or `m5` instances for best RandomX performance

### Linux ARM64 (Graviton)
- Good performance, excellent cost-efficiency
- Hardware AES support on Graviton2+
- RandomX runs well but ~20% slower than equivalent x64

### macOS (Local Development)
- Works on both Intel and Apple Silicon
- Apple Silicon (M1/M2/M3) has excellent RandomX performance
- May need to install Xcode command line tools: `xcode-select --install`

## Troubleshooting

### CGO Errors
```
# Ensure gcc is installed
sudo apt-get install build-essential

# Verify CGO is enabled
CGO_ENABLED=1 go env CGO_ENABLED
```

### Memory Errors (Full Dataset)
```
# Full dataset requires ~2GB RAM
# Use instance with 4GB+ RAM for full tests
# Or skip full dataset test:
./scripts/test-randomx.sh quick
```

### RandomX Build Fails
```
# Ensure cmake is installed
sudo apt-get install cmake

# Clean and rebuild
cd common/randomx
make clean
make randomx
```

## Collecting Results

After running tests, the script generates a report:
```
test-report-YYYYMMDD-HHMMSS.txt
```

Please share this file for each platform you test.

## Instance Launch Commands

### Launch Linux x64 (Ubuntu 22.04)
```bash
aws ec2 run-instances \
  --image-id ami-0c7217cdde317cfec \
  --instance-type t3.medium \
  --key-name your-key \
  --security-groups your-sg \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=opensy-test-x64}]'
```

### Launch Linux ARM64 (Ubuntu 22.04)
```bash
aws ec2 run-instances \
  --image-id ami-0fe630eb857a6ec83 \
  --instance-type t4g.medium \
  --key-name your-key \
  --security-groups your-sg \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=opensy-test-arm64}]'
```

## Cleanup

Don't forget to terminate instances after testing:
```bash
aws ec2 terminate-instances --instance-ids i-xxxxx
```

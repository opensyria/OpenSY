# RandomX CGO Bindings for OpenSY

This package provides Go bindings for the RandomX proof-of-work algorithm used by OpenSY.

## Prerequisites

### Linux (Ubuntu/Debian)
```bash
sudo apt-get install build-essential cmake git
```

### macOS
```bash
xcode-select --install
brew install cmake
```

### Windows
- Install Visual Studio Build Tools 2022
- Install CMake
- Install Git for Windows

## Building RandomX Library

```bash
# From this directory
make randomx

# This will:
# 1. Clone RandomX repository
# 2. Build the static library
# 3. Copy headers to include/
```

## Usage

```go
package main

import (
    "fmt"
    "github.com/opensyria/opensy-mining/common/randomx"
)

func main() {
    // Create a new RandomX context (light mode for validation)
    ctx, err := randomx.NewContext(randomx.FlagDefault)
    if err != nil {
        panic(err)
    }
    defer ctx.Close()

    // Initialize with key (block hash)
    key := []byte("your-32-byte-key-block-hash-here")
    if err := ctx.InitCache(key); err != nil {
        panic(err)
    }

    // Calculate hash
    input := []byte("block header data")
    hash, err := ctx.CalculateHash(input)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Hash: %x\n", hash)
}
```

## Full Dataset Mode (Mining)

For mining, use full dataset mode (~2GB RAM):

```go
ctx, err := randomx.NewContext(randomx.FlagFullMem | randomx.FlagJIT)
if err != nil {
    panic(err)
}
defer ctx.Close()

// Initialize cache and dataset
if err := ctx.InitCache(key); err != nil {
    panic(err)
}
if err := ctx.InitDataset(numThreads); err != nil {
    panic(err)
}

// Create VM for hashing
vm, err := ctx.CreateVM()
if err != nil {
    panic(err)
}
defer vm.Close()

// Hash
hash := vm.CalculateHash(input)
```

## OpenSY-Specific Notes

- **Key Block Interval**: 32 blocks (NOT 2048 like Monero)
- **Key Block Calculation**: `keyHeight = ((height / 32) - 1) * 32`
- **Dataset Regeneration**: Required every ~64 minutes on mainnet

## Thread Safety

- `Context` is NOT thread-safe for initialization
- `VM` instances are NOT thread-safe
- Create one `VM` per goroutine from a shared `Context`
- Multiple VMs can share the same dataset (read-only after init)

## Testing

```bash
go test -v ./...

# With race detector
go test -race -v ./...

# Benchmark
go test -bench=. -benchmem ./...
```

## Platform Support

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux | x86_64 | ✅ Tested |
| Linux | ARM64 | ✅ Tested |
| macOS | x86_64 | ✅ Tested |
| macOS | ARM64 (M1/M2) | ✅ Tested |
| Windows | x86_64 | ⚠️ Requires MinGW or MSVC |

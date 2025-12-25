#!/bin/bash
# OpenSY RandomX CGO Bindings - Multi-Platform Test Script
# 
# This script tests RandomX bindings on different platforms.
# Run this on each AWS instance type to validate builds.
#
# Usage: ./test-randomx.sh [quick|full|bench]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
RANDOMX_DIR="$ROOT_DIR/common/randomx"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_section() { echo -e "\n${BLUE}=== $1 ===${NC}\n"; }

TEST_MODE="${1:-quick}"

# Detect platform
detect_platform() {
    log_section "Platform Detection"
    
    echo "OS:           $(uname -s)"
    echo "Architecture: $(uname -m)"
    echo "Kernel:       $(uname -r)"
    
    if [ -f /etc/os-release ]; then
        echo "Distribution: $(grep PRETTY_NAME /etc/os-release | cut -d'"' -f2)"
    fi
    
    # CPU info
    if [ -f /proc/cpuinfo ]; then
        echo "CPU Model:    $(grep 'model name' /proc/cpuinfo | head -1 | cut -d':' -f2 | xargs)"
        echo "CPU Cores:    $(grep -c processor /proc/cpuinfo)"
    elif command -v sysctl &> /dev/null; then
        echo "CPU Model:    $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'N/A')"
        echo "CPU Cores:    $(sysctl -n hw.ncpu)"
    fi
    
    # Memory
    if [ -f /proc/meminfo ]; then
        MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
        MEM_GB=$((MEM_KB / 1024 / 1024))
        echo "Memory:       ${MEM_GB}GB"
    elif command -v sysctl &> /dev/null; then
        MEM_BYTES=$(sysctl -n hw.memsize 2>/dev/null || echo 0)
        MEM_GB=$((MEM_BYTES / 1024 / 1024 / 1024))
        echo "Memory:       ${MEM_GB}GB"
    fi
    
    # Check for required tools
    echo ""
    echo "Checking dependencies..."
    
    for cmd in go gcc cmake git; do
        if command -v $cmd &> /dev/null; then
            echo "  ✓ $cmd: $(command -v $cmd)"
        else
            echo "  ✗ $cmd: NOT FOUND"
            if [ "$cmd" = "go" ] || [ "$cmd" = "gcc" ]; then
                log_error "$cmd is required. Please install it."
                exit 1
            fi
        fi
    done
    
    # Go version
    echo ""
    echo "Go version: $(go version)"
}

# Build RandomX library
build_randomx() {
    log_section "Building RandomX Library"
    
    cd "$RANDOMX_DIR"
    
    if [ -f "lib/librandomx.a" ] && [ "$TEST_MODE" != "full" ]; then
        log_info "RandomX library exists, skipping build (use 'full' mode to rebuild)"
    else
        log_info "Cloning and building RandomX..."
        make clean 2>/dev/null || true
        make randomx
    fi
    
    # Verify library
    if [ -f "lib/librandomx.a" ]; then
        log_info "Library size: $(ls -lh lib/librandomx.a | awk '{print $5}')"
        log_info "Library built successfully"
    else
        log_error "Failed to build RandomX library"
        exit 1
    fi
    
    cd "$ROOT_DIR"
}

# Run tests
run_tests() {
    log_section "Running Tests"
    
    cd "$RANDOMX_DIR"
    
    log_info "Running unit tests..."
    CGO_ENABLED=1 go test -v ./... 2>&1 | tee test-output.log
    
    TEST_RESULT=$?
    
    if [ $TEST_RESULT -eq 0 ]; then
        log_info "All tests passed!"
    else
        log_error "Some tests failed. Check test-output.log"
        exit 1
    fi
    
    cd "$ROOT_DIR"
}

# Run race detector tests
run_race_tests() {
    log_section "Race Detector Tests"
    
    cd "$RANDOMX_DIR"
    
    log_info "Running tests with race detector..."
    CGO_ENABLED=1 go test -race -v ./... 2>&1 | tee race-test-output.log
    
    cd "$ROOT_DIR"
}

# Run benchmarks
run_benchmarks() {
    log_section "Performance Benchmarks"
    
    cd "$RANDOMX_DIR"
    
    log_info "Running benchmarks (this may take a few minutes)..."
    CGO_ENABLED=1 go test -bench=. -benchmem -benchtime=10s ./... 2>&1 | tee bench-output.log
    
    # Parse and display results
    echo ""
    log_info "Benchmark Results Summary:"
    echo ""
    
    if grep -q "BenchmarkCalculateHash" bench-output.log; then
        HASH_RATE=$(grep "BenchmarkCalculateHash[^P]" bench-output.log | awk '{print $3}' | head -1)
        if [ -n "$HASH_RATE" ]; then
            # Convert ns/op to H/s
            HS=$(echo "scale=2; 1000000000 / $HASH_RATE" | bc 2>/dev/null || echo "N/A")
            echo "  Single-thread hash rate: ~${HS} H/s"
        fi
    fi
    
    if grep -q "BenchmarkCalculateHashParallel" bench-output.log; then
        PARALLEL_RATE=$(grep "BenchmarkCalculateHashParallel" bench-output.log | awk '{print $3}' | head -1)
        if [ -n "$PARALLEL_RATE" ]; then
            HS=$(echo "scale=2; 1000000000 / $PARALLEL_RATE" | bc 2>/dev/null || echo "N/A")
            CORES=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo "?")
            echo "  Parallel hash rate (${CORES} cores): ~${HS} H/s per core"
        fi
    fi
    
    cd "$ROOT_DIR"
}

# Test dataset initialization (requires 2GB+ RAM)
test_full_dataset() {
    log_section "Full Dataset Test"
    
    # Check available memory
    if [ -f /proc/meminfo ]; then
        AVAIL_KB=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
        AVAIL_GB=$((AVAIL_KB / 1024 / 1024))
    else
        AVAIL_GB=4  # Assume enough on macOS
    fi
    
    if [ "$AVAIL_GB" -lt 3 ]; then
        log_warn "Insufficient memory for full dataset test (need 3GB, have ${AVAIL_GB}GB)"
        log_warn "Skipping full dataset test"
        return
    fi
    
    log_info "Testing full dataset initialization (2GB allocation)..."
    
    cd "$RANDOMX_DIR"
    
    # Create a simple test program
    cat > /tmp/dataset_test.go << 'EOF'
package main

import (
    "fmt"
    "time"
    "runtime"
    
    "github.com/opensyria/opensy-mining/common/randomx"
)

func main() {
    fmt.Println("Creating RandomX context with full dataset...")
    
    flags := randomx.GetFlags() | randomx.FlagFullMem | randomx.FlagJIT
    ctx, err := randomx.NewContext(flags)
    if err != nil {
        fmt.Printf("Failed to create context: %v\n", err)
        return
    }
    defer ctx.Close()
    
    key := []byte("test key for dataset init")
    
    fmt.Println("Initializing cache...")
    if err := ctx.InitCache(key); err != nil {
        fmt.Printf("Failed to init cache: %v\n", err)
        return
    }
    
    fmt.Printf("Initializing dataset with %d threads...\n", runtime.NumCPU())
    start := time.Now()
    if err := ctx.InitDataset(0); err != nil {
        fmt.Printf("Failed to init dataset: %v\n", err)
        return
    }
    elapsed := time.Since(start)
    fmt.Printf("Dataset initialized in %v\n", elapsed)
    
    fmt.Println("Creating VM...")
    vm, err := ctx.CreateVM()
    if err != nil {
        fmt.Printf("Failed to create VM: %v\n", err)
        return
    }
    defer vm.Close()
    
    fmt.Println("Testing hash calculation...")
    input := []byte("test input")
    hash := vm.CalculateHash(input)
    fmt.Printf("Hash: %x\n", hash)
    
    fmt.Println("\n✓ Full dataset test passed!")
}
EOF

    # Run the test
    cd "$ROOT_DIR"
    CGO_ENABLED=1 go run /tmp/dataset_test.go
    
    rm /tmp/dataset_test.go
}

# Generate test report
generate_report() {
    log_section "Test Report"
    
    REPORT_FILE="$ROOT_DIR/test-report-$(date +%Y%m%d-%H%M%S).txt"
    
    {
        echo "OpenSY RandomX CGO Bindings - Test Report"
        echo "=========================================="
        echo ""
        echo "Date: $(date)"
        echo "Platform: $(uname -s) $(uname -m)"
        echo "Go Version: $(go version)"
        echo ""
        echo "Test Mode: $TEST_MODE"
        echo ""
        
        if [ -f "$RANDOMX_DIR/test-output.log" ]; then
            echo "Unit Test Results:"
            echo "------------------"
            grep -E "(PASS|FAIL|ok|---)" "$RANDOMX_DIR/test-output.log" || true
            echo ""
        fi
        
        if [ -f "$RANDOMX_DIR/bench-output.log" ]; then
            echo "Benchmark Results:"
            echo "------------------"
            grep "Benchmark" "$RANDOMX_DIR/bench-output.log" || true
            echo ""
        fi
        
    } > "$REPORT_FILE"
    
    log_info "Report saved to: $REPORT_FILE"
}

# Main
main() {
    log_info "OpenSY RandomX CGO Bindings - Multi-Platform Test"
    log_info "Test mode: $TEST_MODE"
    
    detect_platform
    build_randomx
    run_tests
    
    case "$TEST_MODE" in
        quick)
            log_info "Quick tests completed. Use 'full' or 'bench' for more thorough testing."
            ;;
        full)
            run_race_tests
            test_full_dataset
            run_benchmarks
            ;;
        bench)
            run_benchmarks
            ;;
        *)
            log_error "Unknown test mode: $TEST_MODE"
            echo "Usage: $0 [quick|full|bench]"
            exit 1
            ;;
    esac
    
    generate_report
    
    echo ""
    log_info "All tests completed successfully! ✓"
}

main

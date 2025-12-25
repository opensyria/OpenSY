#!/bin/bash
# OpenSY Mining Infrastructure - Build Script
# Usage: ./scripts/build.sh [component] [platform]
#
# Components: all, common, pool, coopmine
# Platforms: local, linux-amd64, linux-arm64, darwin-amd64, darwin-arm64

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

COMPONENT="${1:-all}"
PLATFORM="${2:-local}"

# Ensure Go is installed
if ! command -v go &> /dev/null; then
    log_error "Go is not installed. Please install Go 1.21+."
    exit 1
fi

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
log_info "Using Go version $GO_VERSION"

# Build RandomX library if needed
build_randomx() {
    log_info "Building RandomX library..."
    cd "$ROOT_DIR/common/randomx"
    
    if [ -f "lib/librandomx.a" ]; then
        log_info "RandomX library already exists, skipping..."
    else
        make randomx
    fi
    cd "$ROOT_DIR"
}

# Build common libraries
build_common() {
    log_info "Building common libraries..."
    build_randomx
    
    cd "$ROOT_DIR/common"
    CGO_ENABLED=1 go build ./...
    
    log_info "Running common tests..."
    CGO_ENABLED=1 go test -v ./...
    
    cd "$ROOT_DIR"
}

# Build pool
build_pool() {
    log_info "Building pool..."
    cd "$ROOT_DIR/pool"
    
    case "$PLATFORM" in
        local)
            CGO_ENABLED=1 go build -o bin/opensy-pool ./cmd/pool
            ;;
        linux-amd64)
            GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o bin/linux-amd64/opensy-pool ./cmd/pool
            ;;
        linux-arm64)
            GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build -o bin/linux-arm64/opensy-pool ./cmd/pool
            ;;
        darwin-amd64)
            GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o bin/darwin-amd64/opensy-pool ./cmd/pool
            ;;
        darwin-arm64)
            GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o bin/darwin-arm64/opensy-pool ./cmd/pool
            ;;
        *)
            log_error "Unknown platform: $PLATFORM"
            exit 1
            ;;
    esac
    
    cd "$ROOT_DIR"
}

# Build coopmine
build_coopmine() {
    log_info "Building coopmine..."
    cd "$ROOT_DIR/coopmine"
    
    case "$PLATFORM" in
        local)
            CGO_ENABLED=1 go build -o bin/coopmine-coordinator ./cmd/coordinator
            CGO_ENABLED=1 go build -o bin/coopmine-worker ./cmd/worker
            ;;
        *)
            # Cross-compilation similar to pool
            log_warn "Cross-compilation for coopmine not yet implemented"
            ;;
    esac
    
    cd "$ROOT_DIR"
}

# Main
log_info "OpenSY Mining Build Script"
log_info "Component: $COMPONENT, Platform: $PLATFORM"
echo ""

case "$COMPONENT" in
    all)
        build_common
        build_pool
        build_coopmine
        ;;
    common)
        build_common
        ;;
    pool)
        build_common
        build_pool
        ;;
    coopmine)
        build_common
        build_coopmine
        ;;
    randomx)
        build_randomx
        ;;
    *)
        log_error "Unknown component: $COMPONENT"
        echo "Usage: $0 [all|common|pool|coopmine|randomx] [platform]"
        exit 1
        ;;
esac

log_info "Build completed successfully!"

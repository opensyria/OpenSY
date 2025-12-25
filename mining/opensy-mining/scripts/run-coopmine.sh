#!/bin/bash
# CoopMine Quick Start Script
# Usage: ./run-coopmine.sh [coordinator|worker]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$SCRIPT_DIR/../bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_banner() {
    echo -e "${BLUE}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║           🛠️  CoopMine for OpenSY  🛠️          ║"
    echo "║       Cooperative Mining Made Simple          ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_binaries() {
    if [ ! -f "$BIN_DIR/coordinator" ] || [ ! -f "$BIN_DIR/worker" ]; then
        echo -e "${RED}Error: Binaries not found. Run 'make build' first.${NC}"
        exit 1
    fi
}

run_coordinator() {
    echo -e "${GREEN}Starting CoopMine Coordinator...${NC}"
    echo ""
    
    # Get wallet address
    if [ -z "$WALLET" ]; then
        read -p "Enter your wallet address: " WALLET
    fi
    
    if [ -z "$WALLET" ]; then
        echo -e "${RED}Error: Wallet address is required${NC}"
        exit 1
    fi
    
    # Pool settings
    POOL_ADDR="${POOL:-pool.opensy.network:3333}"
    GRPC_ADDR="${GRPC_ADDR:-0.0.0.0:5555}"
    CLUSTER_NAME="${CLUSTER_NAME:-MyCoopMine}"
    
    echo -e "${YELLOW}Configuration:${NC}"
    echo "  Wallet:       $WALLET"
    echo "  Pool:         $POOL_ADDR"
    echo "  gRPC Listen:  $GRPC_ADDR"
    echo "  Cluster:      $CLUSTER_NAME"
    echo ""
    
    exec "$BIN_DIR/coordinator" \
        --wallet="$WALLET" \
        --pool="$POOL_ADDR" \
        --grpc-addr="$GRPC_ADDR" \
        --cluster-name="$CLUSTER_NAME" \
        --log-level=info
}

run_worker() {
    echo -e "${GREEN}Starting CoopMine Worker...${NC}"
    echo ""
    
    # Coordinator address
    if [ -z "$COORDINATOR" ]; then
        read -p "Enter coordinator address [localhost:5555]: " COORDINATOR
        COORDINATOR="${COORDINATOR:-localhost:5555}"
    fi
    
    # Thread count
    THREADS="${THREADS:-0}"  # 0 = auto-detect
    WORKER_NAME="${WORKER_NAME:-$(hostname)}"
    
    echo -e "${YELLOW}Configuration:${NC}"
    echo "  Coordinator:  $COORDINATOR"
    echo "  Threads:      ${THREADS:-auto}"
    echo "  Worker Name:  $WORKER_NAME"
    echo ""
    
    exec "$BIN_DIR/worker" \
        --coordinator="$COORDINATOR" \
        --threads="$THREADS" \
        --worker-name="$WORKER_NAME" \
        --log-level=info
}

show_help() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  coordinator   Start as coordinator (main machine)"
    echo "  worker        Start as worker (joins coordinator)"
    echo "  help          Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  WALLET         Wallet address (coordinator)"
    echo "  POOL           Pool address (default: pool.opensy.network:3333)"
    echo "  GRPC_ADDR      gRPC listen address (default: 0.0.0.0:5555)"
    echo "  CLUSTER_NAME   Cluster name (default: MyCoopMine)"
    echo "  COORDINATOR    Coordinator address (worker)"
    echo "  THREADS        Mining threads (default: auto)"
    echo "  WORKER_NAME    Worker name (default: hostname)"
    echo ""
    echo "Examples:"
    echo "  # Start coordinator"
    echo "  WALLET=SYxxxxxxxxxx $0 coordinator"
    echo ""
    echo "  # Start worker on another machine"
    echo "  COORDINATOR=192.168.1.100:5555 $0 worker"
    echo ""
}

# Main
print_banner
check_binaries

case "${1:-help}" in
    coordinator|coord|c)
        run_coordinator
        ;;
    worker|work|w)
        run_worker
        ;;
    help|--help|-h|*)
        show_help
        ;;
esac

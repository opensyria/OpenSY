#!/bin/bash
# OpenSY Network Hashrate Monitor
# يعمل على أي node لديها RPC access

set -e

# Configuration
RPC_URL="${RPC_URL:-http://127.0.0.1:9632}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
REFRESH="${REFRESH:-10}"  # seconds

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
WHITE='\033[1;37m'
NC='\033[0m'

rpc_call() {
    local method="$1"
    shift
    local params="${1:-[]}"
    
    local auth=""
    if [ -n "$RPC_USER" ]; then
        auth="-u $RPC_USER:$RPC_PASS"
    fi
    
    curl -s $auth \
        --data-binary "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}" \
        -H "Content-Type: application/json" \
        "$RPC_URL" | jq -r '.result'
}

format_hashrate() {
    local h=$1
    if (( $(echo "$h >= 1000000000" | bc -l) )); then
        echo "$(echo "scale=2; $h / 1000000000" | bc) GH/s"
    elif (( $(echo "$h >= 1000000" | bc -l) )); then
        echo "$(echo "scale=2; $h / 1000000" | bc) MH/s"
    elif (( $(echo "$h >= 1000" | bc -l) )); then
        echo "$(echo "scale=2; $h / 1000" | bc) KH/s"
    else
        echo "${h} H/s"
    fi
}

print_header() {
    clear
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════╗"
    echo "║         📊 OpenSY Network Hashrate Monitor 📊            ║"
    echo "╚═══════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

monitor_loop() {
    while true; do
        print_header
        
        # Get mining info
        local info=$(rpc_call "getmininginfo")
        
        if [ "$info" == "null" ] || [ -z "$info" ]; then
            echo -e "${YELLOW}⚠️  Cannot connect to node at $RPC_URL${NC}"
            echo "Make sure opensyd is running with -server"
            sleep $REFRESH
            continue
        fi
        
        local blocks=$(echo "$info" | jq -r '.blocks')
        local difficulty=$(echo "$info" | jq -r '.difficulty')
        local networkhashps=$(echo "$info" | jq -r '.networkhashps')
        
        # Get blockchain info
        local bcinfo=$(rpc_call "getblockchaininfo")
        local chain=$(echo "$bcinfo" | jq -r '.chain')
        local progress=$(echo "$bcinfo" | jq -r '.verificationprogress')
        
        # Get peer info
        local peers=$(rpc_call "getpeerinfo" | jq 'length')
        
        # Display
        echo -e "${WHITE}Network:${NC}        $chain"
        echo -e "${WHITE}Block Height:${NC}   ${GREEN}$blocks${NC}"
        echo -e "${WHITE}Difficulty:${NC}     $difficulty"
        echo ""
        echo -e "${WHITE}Network Hashrate:${NC}"
        echo -e "  ${GREEN}$(format_hashrate $networkhashps)${NC}"
        echo ""
        echo -e "${WHITE}Connected Peers:${NC} $peers"
        echo -e "${WHITE}Sync Progress:${NC}  $(echo "scale=2; $progress * 100" | bc)%"
        echo ""
        echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        
        # Calculate estimated daily blocks
        # OpenSY: 2 min blocks = 720 blocks/day
        # Your share = (your_hashrate / network_hashrate) * 720
        echo ""
        echo -e "${CYAN}💡 Hashrate Reference (RandomX):${NC}"
        echo "  Raspberry Pi 4:  ~50-100 H/s"
        echo "  Old Laptop:      ~200-500 H/s"
        echo "  Modern Laptop:   ~500-1500 H/s"
        echo "  Desktop (Ryzen): ~2000-6000 H/s"
        echo "  Server:          ~3000-8000 H/s"
        echo ""
        echo -e "${WHITE}Expected with 8 devices: ~5,000-20,000 H/s${NC}"
        echo ""
        echo -e "Refreshing every ${REFRESH}s... (Ctrl+C to exit)"
        
        sleep $REFRESH
    done
}

# Check dependencies
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required. Install with: brew install jq"
    exit 1
fi

if ! command -v bc &> /dev/null; then
    echo "Error: bc is required. Install with: brew install bc"
    exit 1
fi

echo "Connecting to $RPC_URL..."
monitor_loop

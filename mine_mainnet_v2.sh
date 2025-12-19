#!/bin/bash
# OpenSY Mainnet Mining Script v3 - Production Ready
# Usage: ./mine_mainnet_v2.sh [address]

set -euo pipefail

# Configuration
CLI="/Users/hamoudi/OpenSyria/build/bin/opensy-cli"
DAEMON="/Users/hamoudi/OpenSyria/build/bin/opensyd"
ADDR="${1:-syl1qvg2uuau5xegn0nt8fly5m2xm84uvgn3m3aermx}"
LOGFILE="/Users/hamoudi/OpenSyria/mining_v2.log"
STATEFILE="/Users/hamoudi/OpenSyria/mining_state.json"
LOCKFILE="/tmp/opensy_mining.lock"

# Error counter
ERROR_COUNT=0
MAX_ERRORS=10
DAEMON_DOWN_COUNT=0
MAX_DAEMON_DOWN=30  # 5 minutes of daemon being down

# Graceful shutdown flag
SHUTDOWN=0

# Cleanup function
cleanup() {
    log "🛑 Mining stopped gracefully"
    save_state
    rm -f "$LOCKFILE"
    exit 0
}

# Signal handlers for graceful shutdown
trap cleanup SIGINT SIGTERM EXIT

# Logging function
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOGFILE"
}

# Save state to resume later
save_state() {
    if [ -n "${START_HEIGHT:-}" ] && [ -n "${START_TIME:-}" ]; then
        cat > "$STATEFILE" << EOF
{
  "start_height": $START_HEIGHT,
  "start_time": $START_TIME,
  "last_height": $(get_block_count),
  "last_update": $(date +%s)
}
EOF
    fi
}

# Get block count safely
get_block_count() {
    $CLI getblockcount 2>/dev/null || echo "0"
}

# Check if daemon is alive
is_daemon_alive() {
    $CLI getblockcount > /dev/null 2>&1
}

# Verify binaries exist
verify_binaries() {
    if [ ! -f "$CLI" ]; then
        echo "ERROR: CLI binary not found at $CLI"
        exit 1
    fi
    if [ ! -f "$DAEMON" ]; then
        echo "ERROR: Daemon binary not found at $DAEMON"
        exit 1
    fi
    
    # Verify versions match
    CLI_VERSION=$($CLI --version 2>&1 | head -1 | grep -o "v[0-9.]*" || echo "unknown")
    DAEMON_VERSION=$($DAEMON --version 2>&1 | head -1 | grep -o "v[0-9.]*" || echo "unknown")
    
    if [ "$CLI_VERSION" != "$DAEMON_VERSION" ]; then
        echo "WARNING: Version mismatch - CLI: $CLI_VERSION, Daemon: $DAEMON_VERSION"
    fi
}

# Check for existing instance
check_existing_instance() {
    if [ -f "$LOCKFILE" ]; then
        OLD_PID=$(cat "$LOCKFILE")
        if ps -p "$OLD_PID" > /dev/null 2>&1; then
            echo "ERROR: Mining already running (PID $OLD_PID)"
            exit 1
        else
            log "⚠️  Removing stale lock file (PID $OLD_PID no longer running)"
            rm -f "$LOCKFILE"
        fi
    fi
    echo $$ > "$LOCKFILE"
}

# Main execution
main() {
    verify_binaries
    check_existing_instance
    
    log "🚀 OpenSY Mining Started (PID $$)"
    log "   Address: $ADDR"
    log "   CLI: $CLI"
    log "   Version: $($CLI --version 2>&1 | head -1)"
    
    # Load wallet (ignore errors if already loaded)
    $CLI loadwallet "founder" 2>/dev/null || true
    
    # Wait for daemon to be ready
    while ! is_daemon_alive; do
        DAEMON_DOWN_COUNT=$((DAEMON_DOWN_COUNT + 1))
        if [ $DAEMON_DOWN_COUNT -gt $MAX_DAEMON_DOWN ]; then
            log "❌ Daemon not responding after $MAX_DAEMON_DOWN attempts. Exiting."
            exit 1
        fi
        log "⚠️  Waiting for daemon to be ready... ($DAEMON_DOWN_COUNT/$MAX_DAEMON_DOWN)"
        sleep 10
    done
    
    START_HEIGHT=$(get_block_count)
    START_TIME=$(date +%s)
    log "   Starting height: $START_HEIGHT"
    
    # Main mining loop
    while [ $SHUTDOWN -eq 0 ]; do
        # Check daemon health
        if ! is_daemon_alive; then
            DAEMON_DOWN_COUNT=$((DAEMON_DOWN_COUNT + 1))
            log "⚠️  Daemon not responding ($DAEMON_DOWN_COUNT/$MAX_DAEMON_DOWN)"
            
            if [ $DAEMON_DOWN_COUNT -gt $MAX_DAEMON_DOWN ]; then
                log "❌ Daemon down too long. Exiting."
                exit 1
            fi
            
            sleep 10
            continue
        else
            DAEMON_DOWN_COUNT=0
        fi
        
        # Mine block
        RESULT=$($CLI generatetoaddress 1 "$ADDR" 2>&1 || echo "error")
        
        if [[ "$RESULT" == *"error"* ]] || [[ "$RESULT" == "" ]]; then
            ERROR_COUNT=$((ERROR_COUNT + 1))
            log "❌ Mining error ($ERROR_COUNT/$MAX_ERRORS): $RESULT"
            
            if [ $ERROR_COUNT -gt $MAX_ERRORS ]; then
                log "❌ Too many errors. Exiting."
                exit 1
            fi
            
            sleep 5
            continue
        else
            ERROR_COUNT=0  # Reset on success
        fi
        
        # Calculate stats
        CURRENT_HEIGHT=$(get_block_count)
        ELAPSED=$(($(date +%s) - START_TIME))
        TOTAL_MINED=$((CURRENT_HEIGHT - START_HEIGHT))
        
        if [ $ELAPSED -gt 0 ] && [ $TOTAL_MINED -gt 0 ]; then
            RATE=$(echo "scale=2; $TOTAL_MINED * 3600 / $ELAPSED" | bc 2>/dev/null || echo "0")
            BALANCE=$($CLI -rpcwallet=founder getbalance 2>/dev/null || echo "0")
            log "✅ Block $CURRENT_HEIGHT mined | Total: $TOTAL_MINED | Rate: ~$RATE/hr | Balance: $BALANCE SYL"
            
            # Save state periodically (every 10 blocks)
            if [ $((TOTAL_MINED % 10)) -eq 0 ]; then
                save_state
            fi
        fi
    done
}

# Run main function
main

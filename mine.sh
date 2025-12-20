#!/bin/bash
#═══════════════════════════════════════════════════════════════════════════════
#  OpenSY Universal Mining Script v2.0
#═══════════════════════════════════════════════════════════════════════════════
#
#  The ONLY script you need to mine OpenSY on ANY machine.
#  
#  This script handles EVERYTHING automatically:
#  ✅ Installs dependencies if missing (brew/apt)
#  ✅ Clones & builds OpenSY if not found
#  ✅ Starts daemon if not running
#  ✅ Loads existing wallet (prefers "founder")
#  ✅ Handles crashes, restarts, network issues
#  ✅ Works on macOS and Linux
#  ✅ Mines to your specified wallet address
#
#  Usage:
#    ./mine.sh                           # Use default address
#    ./mine.sh <your-address>            # Custom mining address
#    ./mine.sh --install-only            # Just install, don't mine
#    ./mine.sh --check                   # Check status without mining
#
#  One-liner for fresh machines:
#    curl -sL https://raw.githubusercontent.com/opensyria/OpenSY/main/mine.sh | bash
#    curl -sL https://raw.githubusercontent.com/opensyria/OpenSY/main/mine.sh | bash -s <address>
#
#  Environment variables (optional overrides):
#    MINING_ADDRESS      - Wallet address to mine to
#    OPENSY_CLI          - Path to opensy-cli binary
#    OPENSY_DAEMON       - Path to opensyd binary
#    OPENSY_DATADIR      - Data directory path
#    OPENSY_INSTALL_DIR  - Where to clone/build OpenSY
#
#═══════════════════════════════════════════════════════════════════════════════

set -uo pipefail  # Don't use -e, we handle errors ourselves

#───────────────────────────────────────────────────────────────────────────────
# CONFIGURATION
#───────────────────────────────────────────────────────────────────────────────

# Your mining address (override with argument or MINING_ADDRESS env var)
DEFAULT_MINING_ADDRESS="syl1qvg2uuau5xegn0nt8fly5m2xm84uvgn3m3aermx"

# Network settings
MAINNET_PORT=9633
MAINNET_RPCPORT=9632
SEED_NODES="seed.opensyria.net 157.175.40.131"

# Mining settings
BATCH_SIZE=1                    # Blocks per mining call
MINING_DELAY=1                  # Seconds between successful blocks
ERROR_DELAY=10                  # Seconds to wait after error
DAEMON_CHECK_INTERVAL=30        # Seconds between daemon health checks
MAX_CONSECUTIVE_ERRORS=20       # Restart daemon after this many errors
DAEMON_START_TIMEOUT=300        # Max seconds to wait for daemon

# Build settings
BUILD_JOBS=""                   # Auto-detect if empty
SKIP_TESTS=true                 # Skip building tests for faster compile
ENABLE_GUI=false                # Don't build Qt GUI by default

# Logging
LOG_TO_FILE=true
VERBOSE=true

# Script version
VERSION="2.2.0"

# Safety thresholds
MIN_DISK_SPACE_GB=5             # Minimum free disk space to continue
MIN_MEMORY_MB=500               # Minimum free memory to continue
MIN_SYNC_PROGRESS=0.999         # Must be this synced before mining
WAIT_FOR_SYNC=true              # Wait for full sync before mining
AUTO_UPDATE=false               # Auto-update script from GitHub

# Reliability settings
AUTO_RESTART_ON_CRASH=true      # Restart mining if script crashes
MAX_STALE_BLOCK_TIME=600        # Warn if no new block for 10 minutes
NETWORK_RETRY_DELAY=60          # Seconds between network retries
MAX_NETWORK_RETRIES=10          # Max retries before giving up

#───────────────────────────────────────────────────────────────────────────────
# PLATFORM DETECTION
#───────────────────────────────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Darwin*)  echo "macos" ;;
        Linux*)   echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        FreeBSD*) echo "freebsd" ;;
        *)        echo "unknown" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "x64" ;;
        arm64|aarch64) echo "arm64" ;;
        armv7l)        echo "arm32" ;;
        i386|i686)     echo "x86" ;;
        *)             echo "unknown" ;;
    esac
}

detect_package_manager() {
    if command -v brew &>/dev/null; then
        echo "brew"
    elif command -v apt-get &>/dev/null; then
        echo "apt"
    elif command -v dnf &>/dev/null; then
        echo "dnf"
    elif command -v yum &>/dev/null; then
        echo "yum"
    elif command -v pacman &>/dev/null; then
        echo "pacman"
    elif command -v apk &>/dev/null; then
        echo "apk"
    else
        echo "unknown"
    fi
}

OS=$(detect_os)
ARCH=$(detect_arch)
PKG_MGR=$(detect_package_manager)

# Set platform-specific defaults
case "$OS" in
    macos)
        DEFAULT_DATADIR="$HOME/Library/Application Support/OpenSY"
        DEFAULT_INSTALL_DIR="$HOME/OpenSY"
        POSSIBLE_BINARY_DIRS=(
            "$HOME/OpenSyria/build/bin"
            "$HOME/OpenSyria/build_regular/bin"
            "$HOME/OpenSY/build/bin"
            "/opt/opensyria/source/build/bin"
            "/usr/local/bin"
            "/opt/homebrew/bin"
        )
        ;;
    linux)
        DEFAULT_DATADIR="$HOME/.opensy"
        DEFAULT_INSTALL_DIR="$HOME/OpenSY"
        POSSIBLE_BINARY_DIRS=(
            "/opt/opensyria/source/build/bin"
            "$HOME/OpenSY/build/bin"
            "$HOME/OpenSyria/build/bin"
            "/usr/local/bin"
            "/usr/bin"
        )
        ;;
    *)
        DEFAULT_DATADIR="$HOME/.opensy"
        DEFAULT_INSTALL_DIR="$HOME/OpenSY"
        POSSIBLE_BINARY_DIRS=("/usr/local/bin" "/usr/bin")
        ;;
esac

#───────────────────────────────────────────────────────────────────────────────
# GLOBAL STATE
#───────────────────────────────────────────────────────────────────────────────

# Paths (will be set during initialization)
CLI=""
DAEMON=""
DATADIR=""
INSTALL_DIR=""
LOGFILE=""

# Mining state
MINING_ADDRESS=""
WALLET_NAME=""
PREFERRED_WALLETS=("founder" "default" "mining" "main" "")

# Counters
START_TIME=0
START_HEIGHT=0
BLOCKS_MINED=0
ERROR_COUNT=0
TOTAL_ERRORS=0
DAEMON_RESTARTS=0

# Flags
DAEMON_STARTED_BY_US=false
SHUTDOWN_REQUESTED=false
INSTALL_ONLY=false
CHECK_ONLY=false
FORCE_REBUILD=false
SKIP_SCREEN_CHECK=false
RUN_FOREVER=false

# Script paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || pwd)"
SCRIPT_NAME="$(basename "$0")"
LOCKFILE="/tmp/opensy_mine.lock"
PIDFILE="/tmp/opensy_mine.pid"

#───────────────────────────────────────────────────────────────────────────────
# COLORS & LOGGING
#───────────────────────────────────────────────────────────────────────────────

# Check if terminal supports colors
if [ -t 1 ] && command -v tput &>/dev/null && [ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    CYAN='\033[0;36m'
    MAGENTA='\033[0;35m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' CYAN='' MAGENTA='' BOLD='' NC=''
fi

log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local color="" prefix=""
    
    case "$level" in
        INFO)    color="$GREEN";   prefix="✅" ;;
        WARN)    color="$YELLOW";  prefix="⚠️ " ;;
        ERROR)   color="$RED";     prefix="❌" ;;
        DEBUG)   color="$CYAN";    prefix="🔍" ;;
        MINING)  color="$BLUE";    prefix="⛏️ " ;;
        SUCCESS) color="$GREEN";   prefix="🎉" ;;
        BUILD)   color="$MAGENTA"; prefix="🔨" ;;
        INSTALL) color="$CYAN";    prefix="📦" ;;
        *)       color="$NC";      prefix="ℹ️ " ;;
    esac
    
    # Console output
    if [ "$VERBOSE" = true ]; then
        echo -e "${color}${prefix} ${message}${NC}"
    fi
    
    # File output (without colors/emojis)
    if [ "$LOG_TO_FILE" = true ] && [ -n "$LOGFILE" ] && [ -w "$(dirname "$LOGFILE")" ]; then
        echo "[$timestamp] [$level] $message" >> "$LOGFILE" 2>/dev/null || true
    fi
}

die() {
    log ERROR "$1"
    cleanup
    exit 1
}

#───────────────────────────────────────────────────────────────────────────────
# UTILITY FUNCTIONS
#───────────────────────────────────────────────────────────────────────────────

command_exists() {
    command -v "$1" &>/dev/null
}

now() {
    date +%s
}

elapsed_time() {
    local seconds=$1
    local hours=$((seconds / 3600))
    local minutes=$(((seconds % 3600) / 60))
    local secs=$((seconds % 60))
    printf "%02d:%02d:%02d" $hours $minutes $secs
}

get_cpu_cores() {
    if [ "$OS" = "macos" ]; then
        sysctl -n hw.ncpu 2>/dev/null || echo 4
    else
        nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 4
    fi
}

get_available_memory_gb() {
    if [ "$OS" = "macos" ]; then
        echo $(($(sysctl -n hw.memsize 2>/dev/null || echo 8589934592) / 1073741824))
    else
        echo $(($(grep MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo 8388608) / 1048576))
    fi
}

file_size() {
    if [ "$OS" = "macos" ]; then
        stat -f%z "$1" 2>/dev/null || echo 0
    else
        stat -c%s "$1" 2>/dev/null || echo 0
    fi
}

get_free_disk_space_gb() {
    local path="${1:-$HOME}"
    if [ "$OS" = "macos" ]; then
        df -g "$path" 2>/dev/null | awk 'NR==2 {print $4}' || echo 999
    else
        df -BG "$path" 2>/dev/null | awk 'NR==2 {print $4}' | tr -d 'G' || echo 999
    fi
}

check_disk_space() {
    local free=$(get_free_disk_space_gb "$DATADIR")
    if [ "$free" -lt "$MIN_DISK_SPACE_GB" ]; then
        log ERROR "Low disk space: ${free}GB free (minimum: ${MIN_DISK_SPACE_GB}GB)"
        return 1
    fi
    return 0
}

check_internet() {
    # Quick connectivity check (try multiple methods)
    ping -c 1 -W 3 8.8.8.8 &>/dev/null && return 0
    ping -c 1 -W 3 1.1.1.1 &>/dev/null && return 0
    curl -s --connect-timeout 3 https://google.com &>/dev/null && return 0
    return 1
}

get_free_memory_mb() {
    if [ "$OS" = "macos" ]; then
        # macOS: Get free + inactive pages
        local page_size=$(sysctl -n hw.pagesize 2>/dev/null || echo 4096)
        local free=$(vm_stat 2>/dev/null | awk '/Pages free/ {gsub(/\./,"",$3); print $3}')
        local inactive=$(vm_stat 2>/dev/null | awk '/Pages inactive/ {gsub(/\./,"",$3); print $3}')
        echo $(( (${free:-0} + ${inactive:-0}) * page_size / 1048576 ))
    else
        # Linux: MemAvailable or MemFree
        local avail=$(grep MemAvailable /proc/meminfo 2>/dev/null | awk '{print $2}')
        if [ -n "$avail" ]; then
            echo $((avail / 1024))
        else
            grep MemFree /proc/meminfo 2>/dev/null | awk '{print int($2/1024)}' || echo 9999
        fi
    fi
}

check_memory() {
    local free=$(get_free_memory_mb)
    if [ "$free" -lt "$MIN_MEMORY_MB" ]; then
        log WARN "Low memory: ${free}MB free (minimum: ${MIN_MEMORY_MB}MB)"
        return 1
    fi
    return 0
}

wait_for_network() {
    local retries=0
    while ! check_internet; do
        retries=$((retries + 1))
        if [ $retries -ge $MAX_NETWORK_RETRIES ]; then
            log ERROR "Network unreachable after $MAX_NETWORK_RETRIES attempts"
            return 1
        fi
        log WARN "No network connection (attempt $retries/$MAX_NETWORK_RETRIES). Waiting ${NETWORK_RETRY_DELAY}s..."
        sleep $NETWORK_RETRY_DELAY
    done
    return 0
}

check_sudo_available() {
    # Check if we can use sudo (needed for apt install etc)
    if command_exists sudo; then
        if sudo -n true 2>/dev/null; then
            return 0  # Passwordless sudo available
        fi
    fi
    return 1
}

recommend_screen() {
    # Skip if flag is set
    if [ "$SKIP_SCREEN_CHECK" = true ]; then
        return 0
    fi
    
    # Recommend screen/tmux for SSH sessions
    if [ -n "${SSH_CLIENT:-}" ] || [ -n "${SSH_TTY:-}" ]; then
        if [ -z "${STY:-}" ] && [ -z "${TMUX:-}" ]; then
            log WARN "You're connected via SSH without screen/tmux!"
            log WARN "If you disconnect, mining will stop."
            log WARN "Consider running: screen -S mining ./mine.sh"
            log WARN "Or: tmux new -s mining './mine.sh'"
            echo ""
            read -t 10 -p "Continue anyway? (y/N, 10s timeout): " response || response="y"
            if [[ ! "$response" =~ ^[Yy]$ ]]; then
                log INFO "Exiting. Please start in screen/tmux."
                exit 0
            fi
        fi
    fi
}

#───────────────────────────────────────────────────────────────────────────────
# CLEANUP & SIGNAL HANDLING
#───────────────────────────────────────────────────────────────────────────────

cleanup() {
    # Prevent recursive cleanup
    [ "${CLEANUP_DONE:-}" = "true" ] && return
    CLEANUP_DONE=true
    
    log INFO "Shutting down..."
    
    # Print final stats if we were mining
    if [ $START_TIME -gt 0 ]; then
        local elapsed=$(($(now) - START_TIME))
        echo ""
        log INFO "═══════════════════════════════════════════════════════"
        log INFO "Mining Session Summary:"
        log INFO "  Duration: $(elapsed_time $elapsed)"
        log INFO "  Blocks Mined: $BLOCKS_MINED"
        log INFO "  Total Errors: $TOTAL_ERRORS"
        log INFO "  Daemon Restarts: $DAEMON_RESTARTS"
        if [ $elapsed -gt 0 ] && [ $BLOCKS_MINED -gt 0 ]; then
            local rate=$(echo "scale=2; $BLOCKS_MINED * 3600 / $elapsed" | bc 2>/dev/null || echo "N/A")
            log INFO "  Average Rate: $rate blocks/hour"
        fi
        log INFO "═══════════════════════════════════════════════════════"
    fi
    
    # Remove lock files
    rm -f "$LOCKFILE" "$PIDFILE" 2>/dev/null || true
    
    log INFO "Goodbye! 👋"
}

handle_signal() {
    log WARN "Received shutdown signal..."
    SHUTDOWN_REQUESTED=true
}

trap handle_signal SIGINT SIGTERM SIGHUP
trap cleanup EXIT

#───────────────────────────────────────────────────────────────────────────────
# DEPENDENCY INSTALLATION
#───────────────────────────────────────────────────────────────────────────────

check_basic_dependencies() {
    local missing=()
    
    for cmd in git cmake make; do
        if ! command_exists "$cmd"; then
            missing+=("$cmd")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        return 1
    fi
    return 0
}

install_dependencies() {
    log INSTALL "Installing build dependencies for $OS ($PKG_MGR)..."
    
    case "$PKG_MGR" in
        brew)
            # Check if Homebrew is installed
            if ! command_exists brew; then
                log INSTALL "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" || {
                    log ERROR "Failed to install Homebrew"
                    return 1
                }
                # Add to path for this session
                if [ -f /opt/homebrew/bin/brew ]; then
                    eval "$(/opt/homebrew/bin/brew shellenv)"
                elif [ -f /usr/local/bin/brew ]; then
                    eval "$(/usr/local/bin/brew shellenv)"
                fi
            fi
            
            log INSTALL "Installing packages via Homebrew..."
            brew update
            brew install cmake boost libevent miniupnpc zeromq pkg-config autoconf automake libtool || true
            # Optional: Berkeley DB for legacy wallet
            brew install berkeley-db@4 2>/dev/null || true
            ;;
            
        apt)
            log INSTALL "Installing packages via apt..."
            sudo apt-get update
            sudo apt-get install -y \
                build-essential libtool autotools-dev automake pkg-config bsdmainutils python3 \
                libevent-dev libboost-dev libboost-system-dev libboost-filesystem-dev \
                libboost-thread-dev libboost-chrono-dev libboost-program-options-dev \
                libsqlite3-dev libminiupnpc-dev libnatpmp-dev libzmq3-dev \
                cmake git curl wget \
                || return 1
            ;;
            
        dnf|yum)
            log INSTALL "Installing packages via $PKG_MGR..."
            sudo $PKG_MGR groupinstall -y "Development Tools"
            sudo $PKG_MGR install -y \
                cmake boost-devel libevent-devel miniupnpc-devel zeromq-devel \
                openssl-devel git curl \
                || return 1
            ;;
            
        pacman)
            log INSTALL "Installing packages via pacman..."
            sudo pacman -Sy --noconfirm \
                base-devel cmake boost libevent miniupnpc zeromq git \
                || return 1
            ;;
            
        apk)
            log INSTALL "Installing packages via apk..."
            sudo apk add --no-cache \
                build-base cmake boost-dev libevent-dev miniupnpc-dev zeromq-dev git \
                || return 1
            ;;
            
        *)
            log WARN "Unknown package manager. Please install manually:"
            log WARN "  cmake, boost, libevent, miniupnpc, zeromq, git"
            return 1
            ;;
    esac
    
    log SUCCESS "Dependencies installed!"
    return 0
}

#───────────────────────────────────────────────────────────────────────────────
# BINARY DISCOVERY & BUILDING
#───────────────────────────────────────────────────────────────────────────────

find_binaries() {
    log DEBUG "Searching for OpenSY binaries..."
    
    # Check environment variables first
    if [ -n "${OPENSY_CLI:-}" ] && [ -x "$OPENSY_CLI" ]; then
        CLI="$OPENSY_CLI"
    fi
    if [ -n "${OPENSY_DAEMON:-}" ] && [ -x "$OPENSY_DAEMON" ]; then
        DAEMON="$OPENSY_DAEMON"
    fi
    
    # Search in known locations
    for dir in "${POSSIBLE_BINARY_DIRS[@]}"; do
        [ -d "$dir" ] || continue
        
        if [ -z "$CLI" ] && [ -x "$dir/opensy-cli" ]; then
            CLI="$dir/opensy-cli"
        fi
        if [ -z "$DAEMON" ] && [ -x "$dir/opensyd" ]; then
            DAEMON="$dir/opensyd"
        fi
    done
    
    # Check script directory
    if [ -z "$CLI" ] && [ -x "$SCRIPT_DIR/build/bin/opensy-cli" ]; then
        CLI="$SCRIPT_DIR/build/bin/opensy-cli"
    fi
    if [ -z "$DAEMON" ] && [ -x "$SCRIPT_DIR/build/bin/opensyd" ]; then
        DAEMON="$SCRIPT_DIR/build/bin/opensyd"
    fi
    
    # Check if we found both
    if [ -n "$CLI" ] && [ -n "$DAEMON" ]; then
        log INFO "Found binaries:"
        log INFO "  CLI:    $CLI"
        log INFO "  Daemon: $DAEMON"
        return 0
    else
        log DEBUG "Binaries not found in standard locations"
        return 1
    fi
}

clone_repository() {
    INSTALL_DIR="${OPENSY_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
    
    if [ -d "$INSTALL_DIR/.git" ]; then
        log BUILD "Updating existing repository..."
        cd "$INSTALL_DIR"
        git fetch origin main 2>/dev/null || true
        git reset --hard origin/main 2>/dev/null || git pull origin main || true
    else
        log BUILD "Cloning OpenSY repository..."
        rm -rf "$INSTALL_DIR" 2>/dev/null || true
        git clone --depth 1 https://github.com/opensyria/OpenSY.git "$INSTALL_DIR" || {
            # Try alternate URL
            git clone --depth 1 https://github.com/opensyria/OpenSyria.git "$INSTALL_DIR" || {
                log ERROR "Failed to clone repository"
                return 1
            }
        }
        cd "$INSTALL_DIR"
    fi
    
    log SUCCESS "Repository ready at $INSTALL_DIR"
    return 0
}

build_opensy() {
    log BUILD "Building OpenSY (this may take 10-30 minutes)..."
    
    cd "$INSTALL_DIR"
    
    # Determine build parallelism
    local cores=$(get_cpu_cores)
    local mem=$(get_available_memory_gb)
    
    # Each compile job needs ~2GB RAM, limit accordingly
    local max_by_mem=$((mem / 2))
    [ $max_by_mem -lt 1 ] && max_by_mem=1
    
    if [ -n "$BUILD_JOBS" ]; then
        local jobs=$BUILD_JOBS
    else
        local jobs=$((cores > max_by_mem ? max_by_mem : cores))
        [ $jobs -lt 1 ] && jobs=1
    fi
    
    log BUILD "Using $jobs parallel build jobs (cores=$cores, mem=${mem}GB)"
    
    # Create build directory
    local BUILD_DIR="$INSTALL_DIR/build"
    mkdir -p "$BUILD_DIR"
    cd "$BUILD_DIR"
    
    # Configure
    log BUILD "Configuring..."
    local cmake_opts=(
        -DBUILD_TESTS=OFF
        -DENABLE_GUI=OFF
        -DWITH_MINIUPNPC=ON
        -DWITH_ZMQ=ON
    )
    
    # Platform-specific options
    if [ "$OS" = "macos" ] && [ "$ARCH" = "arm64" ]; then
        cmake_opts+=(-DCMAKE_OSX_ARCHITECTURES=arm64)
    fi
    
    cmake .. "${cmake_opts[@]}" 2>&1 | tail -10 || {
        log ERROR "CMake configuration failed"
        return 1
    }
    
    # Build
    log BUILD "Compiling (please wait)..."
    cmake --build . -j$jobs 2>&1 | tail -20 || {
        log ERROR "Build failed"
        log ERROR "Check the full log or try: cd $BUILD_DIR && cmake --build . -j1"
        return 1
    }
    
    # Verify
    if [ ! -x "$BUILD_DIR/bin/opensyd" ] || [ ! -x "$BUILD_DIR/bin/opensy-cli" ]; then
        log ERROR "Build completed but binaries not found"
        return 1
    fi
    
    CLI="$BUILD_DIR/bin/opensy-cli"
    DAEMON="$BUILD_DIR/bin/opensyd"
    
    log SUCCESS "Build complete!"
    log INFO "  CLI:    $CLI"
    log INFO "  Daemon: $DAEMON"
    
    return 0
}

ensure_binaries() {
    # Try to find existing binaries
    if find_binaries && [ "$FORCE_REBUILD" != true ]; then
        return 0
    fi
    
    log INFO "OpenSY binaries not found. Will build from source..."
    
    # Check/install dependencies
    if ! check_basic_dependencies; then
        log INSTALL "Installing required dependencies..."
        install_dependencies || die "Failed to install dependencies"
    fi
    
    # Clone repository
    clone_repository || die "Failed to clone repository"
    
    # Build
    build_opensy || die "Failed to build OpenSY"
    
    return 0
}

#───────────────────────────────────────────────────────────────────────────────
# DATA DIRECTORY & LOGGING SETUP
#───────────────────────────────────────────────────────────────────────────────

setup_datadir() {
    DATADIR="${OPENSY_DATADIR:-$DEFAULT_DATADIR}"
    
    # Create if doesn't exist
    if ! mkdir -p "$DATADIR" 2>/dev/null; then
        log WARN "Cannot create $DATADIR, trying home directory..."
        DATADIR="$HOME/.opensy"
        mkdir -p "$DATADIR" || die "Cannot create data directory"
    fi
    
    log DEBUG "Data directory: $DATADIR"
}

setup_logging() {
    # Determine log file location
    if [ -n "${OPENSY_LOGFILE:-}" ]; then
        LOGFILE="$OPENSY_LOGFILE"
    elif [ -n "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
        LOGFILE="$INSTALL_DIR/mine.log"
    elif [ -w "$SCRIPT_DIR" ]; then
        LOGFILE="$SCRIPT_DIR/mine.log"
    else
        LOGFILE="$HOME/opensy_mine.log"
    fi
    
    # Create directory if needed
    mkdir -p "$(dirname "$LOGFILE")" 2>/dev/null || true
    
    # Rotate if too large (>10MB)
    if [ -f "$LOGFILE" ] && [ $(file_size "$LOGFILE") -gt 10485760 ]; then
        mv "$LOGFILE" "${LOGFILE}.old" 2>/dev/null || true
    fi
    
    log DEBUG "Logging to: $LOGFILE"
}

#───────────────────────────────────────────────────────────────────────────────
# DAEMON MANAGEMENT
#───────────────────────────────────────────────────────────────────────────────

cli_call() {
    "$CLI" -datadir="$DATADIR" "$@" 2>/dev/null
}

is_daemon_running() {
    # Check if we can communicate with daemon
    cli_call getblockcount &>/dev/null
}

is_daemon_process_running() {
    # Check if daemon process exists
    pgrep -f "opensyd.*-datadir" &>/dev/null || pgrep -x opensyd &>/dev/null
}

wait_for_daemon() {
    local timeout=${1:-$DAEMON_START_TIMEOUT}
    local elapsed=0
    
    log INFO "Waiting for daemon to be ready (timeout: ${timeout}s)..."
    
    while [ $elapsed -lt $timeout ]; do
        if is_daemon_running; then
            log SUCCESS "Daemon is ready!"
            return 0
        fi
        
        # Check if process died
        if [ $elapsed -gt 10 ] && ! is_daemon_process_running; then
            log ERROR "Daemon process died during startup"
            return 1
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        
        # Progress indicator
        if [ $((elapsed % 30)) -eq 0 ]; then
            log DEBUG "Still waiting... ($elapsed/$timeout seconds)"
        fi
    done
    
    log ERROR "Daemon startup timeout after $timeout seconds"
    return 1
}

start_daemon() {
    log INFO "Starting OpenSY daemon..."
    
    # Check if already running
    if is_daemon_running; then
        log INFO "Daemon is already running"
        return 0
    fi
    
    # Kill any zombie processes
    if is_daemon_process_running; then
        log WARN "Found zombie daemon process, killing..."
        pkill -9 -f "opensyd" 2>/dev/null || true
        sleep 2
    fi
    
    # Build command with seed nodes
    local addnodes=""
    for node in $SEED_NODES; do
        addnodes="$addnodes -addnode=$node"
    done
    
    # Create config file if it doesn't exist
    local conf_file="$DATADIR/opensy.conf"
    if [ ! -f "$conf_file" ]; then
        log DEBUG "Creating config file..."
        cat > "$conf_file" << EOF
# OpenSY Configuration
server=1
listen=1
daemon=1
rpcallowip=127.0.0.1
rpcbind=127.0.0.1
EOF
        for node in $SEED_NODES; do
            echo "addnode=$node" >> "$conf_file"
        done
    fi
    
    # Start daemon
    log DEBUG "Launching: $DAEMON -datadir=$DATADIR -daemon"
    "$DAEMON" -datadir="$DATADIR" -daemon $addnodes 2>&1 || {
        log ERROR "Failed to launch daemon"
        return 1
    }
    
    DAEMON_STARTED_BY_US=true
    DAEMON_RESTARTS=$((DAEMON_RESTARTS + 1))
    
    # Wait for it to be ready
    wait_for_daemon || return 1
    
    return 0
}

stop_daemon() {
    if [ "$DAEMON_STARTED_BY_US" = true ]; then
        log INFO "Stopping daemon..."
        cli_call stop 2>/dev/null || true
        sleep 3
    fi
}

ensure_daemon_running() {
    if ! is_daemon_running; then
        log WARN "Daemon not responding, attempting restart..."
        start_daemon || return 1
    fi
    return 0
}

get_block_count() {
    cli_call getblockcount 2>/dev/null || echo "0"
}

get_connection_count() {
    cli_call getconnectioncount 2>/dev/null || echo "0"
}

get_network_info() {
    cli_call getnetworkinfo 2>/dev/null
}

get_sync_progress() {
    local info=$(cli_call getblockchaininfo 2>/dev/null)
    if [ -n "$info" ]; then
        echo "$info" | grep -o '"verificationprogress":[^,]*' | cut -d: -f2 | tr -d ' ' || echo "1"
    else
        echo "0"
    fi
}

is_initial_block_download() {
    local info=$(cli_call getblockchaininfo 2>/dev/null)
    if echo "$info" | grep -q '"initialblockdownload": *true'; then
        return 0  # Still in IBD
    fi
    return 1  # Not in IBD
}

wait_for_sync() {
    if [ "$WAIT_FOR_SYNC" != true ]; then
        return 0
    fi
    
    log INFO "Checking sync status..."
    
    local max_wait=3600  # Max 1 hour
    local waited=0
    
    while is_initial_block_download && [ $waited -lt $max_wait ]; do
        local progress=$(get_sync_progress)
        local height=$(get_block_count)
        log INFO "Syncing: ${progress} (height: $height) - waiting..."
        sleep 30
        waited=$((waited + 30))
    done
    
    if is_initial_block_download; then
        log WARN "Still syncing after ${max_wait}s, proceeding anyway..."
    else
        log SUCCESS "Node is fully synced!"
    fi
}

#───────────────────────────────────────────────────────────────────────────────
# WALLET MANAGEMENT
#───────────────────────────────────────────────────────────────────────────────

setup_wallet() {
    log INFO "Setting up wallet..."
    
    # Check if any wallet is already loaded
    local loaded=$(cli_call listwallets 2>/dev/null)
    if [ -n "$loaded" ] && [ "$loaded" != "[]" ]; then
        WALLET_NAME=$(echo "$loaded" | tr -d '[]" \n' | cut -d',' -f1)
        log SUCCESS "Using already loaded wallet: ${WALLET_NAME:-default}"
        return 0
    fi
    
    # Try to load preferred wallets in order
    for name in "${PREFERRED_WALLETS[@]}"; do
        if cli_call loadwallet "$name" &>/dev/null; then
            WALLET_NAME="$name"
            log SUCCESS "Loaded wallet: ${WALLET_NAME:-default}"
            return 0
        fi
    done
    
    # List available wallets in wallet directory
    local wallet_dir=$(cli_call listwalletdir 2>/dev/null)
    if [ -n "$wallet_dir" ]; then
        local first_wallet=$(echo "$wallet_dir" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$first_wallet" ]; then
            if cli_call loadwallet "$first_wallet" &>/dev/null; then
                WALLET_NAME="$first_wallet"
                log SUCCESS "Loaded wallet: $WALLET_NAME"
                return 0
            fi
        fi
    fi
    
    # Create new wallet only as last resort
    log WARN "No existing wallet found, creating 'founder' wallet..."
    if cli_call createwallet "founder" &>/dev/null; then
        WALLET_NAME="founder"
        log SUCCESS "Created wallet: $WALLET_NAME"
        return 0
    fi
    
    log WARN "Could not setup wallet, mining to external address should still work"
    return 0
}

get_balance() {
    if [ -n "$WALLET_NAME" ]; then
        cli_call -rpcwallet="$WALLET_NAME" getbalance 2>/dev/null || echo "N/A"
    else
        cli_call getbalance 2>/dev/null || echo "N/A"
    fi
}

get_wallet_address() {
    if [ -n "$WALLET_NAME" ]; then
        cli_call -rpcwallet="$WALLET_NAME" getnewaddress 2>/dev/null
    else
        cli_call getnewaddress 2>/dev/null
    fi
}

#───────────────────────────────────────────────────────────────────────────────
# ADDRESS VALIDATION
#───────────────────────────────────────────────────────────────────────────────

validate_address() {
    local addr="$1"
    
    # Empty check
    [ -z "$addr" ] && return 1
    
    # Format validation (bech32 or legacy)
    if [[ "$addr" =~ ^syl1[a-z0-9]{39,59}$ ]]; then
        return 0  # Valid bech32
    elif [[ "$addr" =~ ^[SF][a-zA-Z0-9]{33}$ ]]; then
        return 0  # Valid legacy (S or F prefix)
    fi
    
    # RPC validation as fallback
    local result=$(cli_call validateaddress "$addr" 2>/dev/null)
    if echo "$result" | grep -q '"isvalid": *true'; then
        return 0
    fi
    
    return 1
}

#───────────────────────────────────────────────────────────────────────────────
# MINING FUNCTIONS
#───────────────────────────────────────────────────────────────────────────────

mine_block() {
    local result
    result=$(cli_call generatetoaddress $BATCH_SIZE "$MINING_ADDRESS" 2>&1)
    local exit_code=$?
    
    if [ $exit_code -eq 0 ] && [[ "$result" =~ ^\[.*\]$ ]]; then
        return 0
    else
        echo "$result"
        return 1
    fi
}

show_stats() {
    local current_height=$1
    local elapsed=$(($(now) - START_TIME))
    
    if [ $elapsed -gt 0 ]; then
        local rate=""
        if [ $BLOCKS_MINED -gt 0 ]; then
            rate=$(echo "scale=2; $BLOCKS_MINED * 3600 / $elapsed" | bc 2>/dev/null || echo "?")
        else
            rate="0"
        fi
        local balance=$(get_balance)
        local connections=$(get_connection_count)
        local uptime=$(elapsed_time $elapsed)
        local free_space=$(get_free_disk_space_gb "$DATADIR")
        
        log MINING "Height: $current_height | Mined: $BLOCKS_MINED | Rate: ${rate}/hr | Balance: $balance SYL | Peers: $connections | Disk: ${free_space}GB | Up: $uptime"
    fi
}

# Reward tracking
get_block_reward() {
    local height=${1:-0}
    # OpenSY halving schedule: every 420,000 blocks
    local halvings=$((height / 420000))
    local reward=5000  # Initial reward
    for ((i=0; i<halvings && i<64; i++)); do
        reward=$((reward / 2))
    done
    echo $reward
}

mining_loop() {
    echo ""
    log INFO "═══════════════════════════════════════════════════════════════"
    log INFO "Starting mining loop"
    log INFO "  Address: $MINING_ADDRESS"
    log INFO "  Batch: $BATCH_SIZE block(s) per round"
    log INFO "  Wallet: ${WALLET_NAME:-external}"
    log INFO "═══════════════════════════════════════════════════════════════"
    echo ""
    
    START_TIME=$(now)
    START_HEIGHT=$(get_block_count)
    
    log INFO "Starting at block height: $START_HEIGHT"
    
    local last_daemon_check=$(now)
    local last_stats_time=$(now)
    local last_block_time=$(now)
    local last_block_height=$START_HEIGHT
    
    while [ "$SHUTDOWN_REQUESTED" = false ]; do
        local current_time=$(now)
        
        # Periodic daemon health check
        if [ $((current_time - last_daemon_check)) -gt $DAEMON_CHECK_INTERVAL ]; then
            if ! ensure_daemon_running; then
                log ERROR "Cannot reach daemon, waiting..."
                sleep $ERROR_DELAY
                continue
            fi
            last_daemon_check=$current_time
        fi
        
        # Attempt to mine
        local error_output
        if error_output=$(mine_block); then
            # Success!
            ERROR_COUNT=0
            BLOCKS_MINED=$((BLOCKS_MINED + BATCH_SIZE))
            
            local current_height=$(get_block_count)
            show_stats $current_height
            
            # Track last successful block
            last_block_time=$(now)
            last_block_height=$current_height
            
            sleep $MINING_DELAY
        else
            # Error
            ERROR_COUNT=$((ERROR_COUNT + 1))
            TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
            
            # Only log errors periodically to avoid spam
            if [ $ERROR_COUNT -eq 1 ] || [ $((ERROR_COUNT % 5)) -eq 0 ]; then
                log WARN "Mining error ($ERROR_COUNT/$MAX_CONSECUTIVE_ERRORS): $(echo "$error_output" | head -1)"
            fi
            
            # Too many errors - check daemon
            if [ $ERROR_COUNT -ge $MAX_CONSECUTIVE_ERRORS ]; then
                log ERROR "Too many consecutive errors, checking daemon..."
                
                if ! ensure_daemon_running; then
                    log ERROR "Daemon unrecoverable. Waiting longer..."
                    sleep 60
                fi
                
                ERROR_COUNT=0
            fi
            
            sleep $ERROR_DELAY
        fi
        
        # Periodic stats (every 5 minutes even if no blocks)
        if [ $((current_time - last_stats_time)) -gt 300 ]; then
            show_stats $(get_block_count)
            last_stats_time=$current_time
            
            # Periodic health checks
            if ! check_disk_space; then
                log ERROR "Disk space critically low! Pausing mining..."
                sleep 300  # Wait 5 minutes before checking again
            fi
            
            # Memory check
            if ! check_memory; then
                log WARN "Memory low, daemon may be swapping. Consider restarting."
            fi
            
            # Stale block detection
            local time_since_block=$((current_time - last_block_time))
            if [ $time_since_block -gt $MAX_STALE_BLOCK_TIME ]; then
                log WARN "No blocks mined for $(elapsed_time $time_since_block). Network difficulty may have increased."
                # Check if blockchain is actually progressing
                local network_height=$(get_block_count)
                if [ "$network_height" != "$last_block_height" ]; then
                    log WARN "Network height is $network_height but we haven't mined. Competition?"
                fi
            fi
            
            # Network connectivity check
            if ! check_internet; then
                log ERROR "Network connectivity lost!"
                wait_for_network || log ERROR "Network still down, continuing anyway..."
            fi
        fi
    done
    
    log INFO "Mining loop ended gracefully"
    return 0
}

#───────────────────────────────────────────────────────────────────────────────
# LOCK FILE MANAGEMENT
#───────────────────────────────────────────────────────────────────────────────

check_already_running() {
    if [ -f "$LOCKFILE" ]; then
        local old_pid=$(cat "$LOCKFILE" 2>/dev/null)
        if [ -n "$old_pid" ] && kill -0 "$old_pid" 2>/dev/null; then
            log ERROR "Mining already running (PID $old_pid)"
            log ERROR "To stop it: kill $old_pid"
            log ERROR "To force restart: rm $LOCKFILE && ./mine.sh"
            exit 1
        fi
        log DEBUG "Removing stale lock file"
        rm -f "$LOCKFILE"
    fi
    
    echo $$ > "$LOCKFILE"
    echo $$ > "$PIDFILE"
}

#───────────────────────────────────────────────────────────────────────────────
# STATUS CHECK
#───────────────────────────────────────────────────────────────────────────────

show_status() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║              OpenSY Status Check                                  ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    
    echo "System:"
    echo "  OS:      $OS ($ARCH)"
    echo "  Package: $PKG_MGR"
    echo "  Cores:   $(get_cpu_cores)"
    echo "  Memory:  $(get_available_memory_gb) GB"
    echo ""
    
    echo "Binaries:"
    if find_binaries; then
        echo "  CLI:     $CLI ✅"
        echo "  Daemon:  $DAEMON ✅"
    else
        echo "  Status:  Not found ❌"
        echo "  Action:  Run ./mine.sh to auto-install"
    fi
    echo ""
    
    setup_datadir
    echo "Data Directory:"
    echo "  Path:    $DATADIR"
    if [ -d "$DATADIR" ]; then
        echo "  Status:  Exists ✅"
    else
        echo "  Status:  Does not exist"
    fi
    echo ""
    
    echo "Daemon:"
    if is_daemon_running; then
        echo "  Status:  Running ✅"
        echo "  Height:  $(get_block_count)"
        echo "  Peers:   $(get_connection_count)"
        echo "  Sync:    $(get_sync_progress)"
        
        setup_wallet
        echo ""
        echo "Wallet:"
        echo "  Name:    ${WALLET_NAME:-default}"
        echo "  Balance: $(get_balance) SYL"
    else
        echo "  Status:  Not running ❌"
    fi
    echo ""
}

#───────────────────────────────────────────────────────────────────────────────
# HELP
#───────────────────────────────────────────────────────────────────────────────

show_help() {
    cat << EOF
OpenSY Universal Mining Script v$VERSION

Usage: $SCRIPT_NAME [OPTIONS] [ADDRESS]

Arguments:
  ADDRESS               Mining address (default: $DEFAULT_MINING_ADDRESS)

Options:
  --help, -h           Show this help message
  --check, -c          Check system status without mining
  --install-only, -i   Install/build OpenSY without mining
  --rebuild, -r        Force rebuild even if binaries exist
  --quiet, -q          Less verbose output
  --wait-sync, -w      Wait for full sync before mining
  --no-wait-sync       Start mining immediately (may orphan blocks)
  --no-screen-check    Skip screen/tmux recommendation for SSH
  --loop, --forever    Auto-restart if script crashes (run until killed)
  --version, -v        Show version

Environment Variables:
  MINING_ADDRESS       Override mining address
  OPENSY_CLI           Path to opensy-cli binary
  OPENSY_DAEMON        Path to opensyd binary  
  OPENSY_DATADIR       Data directory path
  OPENSY_INSTALL_DIR   Where to clone/build OpenSY

Examples:
  $SCRIPT_NAME                                    # Mine with default address
  $SCRIPT_NAME syl1abc123...                      # Mine to specific address
  $SCRIPT_NAME --check                            # Check status
  $SCRIPT_NAME --install-only                     # Just install
  curl -sL .../mine.sh | bash                     # One-liner install & mine
  curl -sL .../mine.sh | bash -s -- syl1abc...    # One-liner with address

EOF
}

#───────────────────────────────────────────────────────────────────────────────
# ARGUMENT PARSING
#───────────────────────────────────────────────────────────────────────────────

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --help|-h)
                show_help
                exit 0
                ;;
            --version|-v)
                echo "OpenSY Mining Script v$VERSION"
                exit 0
                ;;
            --check|-c)
                CHECK_ONLY=true
                shift
                ;;
            --install-only|-i)
                INSTALL_ONLY=true
                shift
                ;;
            --rebuild|-r)
                FORCE_REBUILD=true
                shift
                ;;
            --quiet|-q)
                VERBOSE=false
                shift
                ;;
            --wait-sync|-w)
                WAIT_FOR_SYNC=true
                shift
                ;;
            --no-wait-sync)
                WAIT_FOR_SYNC=false
                shift
                ;;
            --no-screen-check)
                SKIP_SCREEN_CHECK=true
                shift
                ;;
            --loop|--forever)
                RUN_FOREVER=true
                shift
                ;;
            --*)
                log ERROR "Unknown option: $1"
                show_help
                exit 1
                ;;
            *)
                # Assume it's an address
                MINING_ADDRESS="$1"
                shift
                ;;
        esac
    done
}

#───────────────────────────────────────────────────────────────────────────────
# MAIN
#───────────────────────────────────────────────────────────────────────────────

main() {
    # Parse command line arguments
    parse_args "$@"
    
    # Set mining address from environment or default
    MINING_ADDRESS="${MINING_ADDRESS:-${MINING_ADDRESS_ENV:-$DEFAULT_MINING_ADDRESS}}"
    # Check env var with proper name
    MINING_ADDRESS="${MINING_ADDRESS:-$DEFAULT_MINING_ADDRESS}"
    
    # Banner
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║         OpenSY Universal Mining Script v$VERSION                    ║"
    echo "║         🇸🇾 Syria's First Cryptocurrency                           ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    
    # Status check mode
    if [ "$CHECK_ONLY" = true ]; then
        show_status
        exit 0
    fi
    
    # Setup logging early
    setup_logging
    
    # Check for existing instance
    check_already_running
    
    # Recommend screen/tmux for SSH sessions
    recommend_screen
    
    log INFO "Platform: $OS ($ARCH) - Package Manager: $PKG_MGR"
    
    # Pre-flight network check
    if ! check_internet; then
        log WARN "No network connection detected. Will retry..."
        wait_for_network || die "Cannot proceed without network"
    fi
    
    # Ensure we have binaries (install if needed)
    ensure_binaries
    
    # Install-only mode
    if [ "$INSTALL_ONLY" = true ]; then
        log SUCCESS "Installation complete!"
        log INFO "Binaries installed at:"
        log INFO "  CLI:    $CLI"
        log INFO "  Daemon: $DAEMON"
        log INFO ""
        log INFO "To start mining: $SCRIPT_NAME"
        exit 0
    fi
    
    # Setup data directory
    setup_datadir
    log INFO "Data directory: $DATADIR"
    
    # Validate mining address
    if ! validate_address "$MINING_ADDRESS"; then
        log WARN "Address validation uncertain: $MINING_ADDRESS"
        log WARN "Proceeding anyway - daemon will validate"
    fi
    log INFO "Mining address: $MINING_ADDRESS"
    
    # Ensure daemon is running
    if ! ensure_daemon_running; then
        die "Failed to start daemon"
    fi
    
    # Show node status
    local height=$(get_block_count)
    local connections=$(get_connection_count)
    local progress=$(get_sync_progress)
    log INFO "Node status: Height=$height Peers=$connections Sync=$progress"
    
    # Warn if low peer count
    if [ "$connections" = "0" ]; then
        log WARN "No peers connected! Mining may produce orphan blocks."
        log WARN "Adding seed nodes..."
        for node in $SEED_NODES; do
            cli_call addnode "$node" onetry &>/dev/null || true
        done
    fi
    
    # Setup wallet
    setup_wallet
    
    # Wait for sync if configured
    if is_initial_block_download; then
        if [ "$WAIT_FOR_SYNC" = true ]; then
            wait_for_sync
        else
            log WARN "Node is still syncing (IBD). Blocks may be orphaned."
            log WARN "Use --wait-sync to wait for full sync before mining."
        fi
    fi
    
    # Final pre-flight checks
    if ! check_disk_space; then
        die "Insufficient disk space to start mining"
    fi
    
    if ! check_memory; then
        log WARN "Low memory, but continuing. Performance may be degraded."
    fi
    
    # Display final summary before mining
    echo ""
    log INFO "Pre-flight checks passed! Starting mining in 3 seconds..."
    log INFO "  Address: $MINING_ADDRESS"
    log INFO "  Wallet:  ${WALLET_NAME:-external}"
    log INFO "  Height:  $(get_block_count)"
    log INFO "  Peers:   $(get_connection_count)"
    log INFO "Press Ctrl+C to stop mining gracefully."
    sleep 3
    
    # Start mining (with auto-restart if --loop)
    if [ "$RUN_FOREVER" = true ]; then
        run_forever
    else
        mining_loop
    fi
}

# Run with auto-restart wrapper if --loop is set
run_forever() {
    local stop_file="/tmp/opensy_stop_mining"
    rm -f "$stop_file" 2>/dev/null
    
    log INFO "Running in forever mode. To stop: touch $stop_file"
    
    local restart_count=0
    local last_restart=$(now)
    
    while [ ! -f "$stop_file" ]; do
        restart_count=$((restart_count + 1))
        
        if [ $restart_count -gt 1 ]; then
            # Exponential backoff: 10s, 20s, 40s, max 300s
            local backoff=$((10 * (2 ** (restart_count - 2))))
            [ $backoff -gt 300 ] && backoff=300
            log WARN "Restarting (attempt $restart_count) in ${backoff}s..."
            sleep $backoff
        fi
        
        log INFO "Starting mining session #$restart_count..."
        
        # Reset flags for fresh run
        SHUTDOWN_REQUESTED=false
        CLEANUP_DONE=""
        START_TIME=0
        BLOCKS_MINED=0
        ERROR_COUNT=0
        TOTAL_ERRORS=0
        
        # Run mining
        if mining_loop; then
            # Graceful exit
            log INFO "Mining stopped gracefully."
            break
        else
            log ERROR "Mining crashed! Will restart..."
        fi
        
        # Reset restart counter if we ran for more than 1 hour
        local now_time=$(now)
        if [ $((now_time - last_restart)) -gt 3600 ]; then
            restart_count=1
        fi
        last_restart=$now_time
    done
    
    if [ -f "$stop_file" ]; then
        log INFO "Stop file detected, exiting forever loop."
        rm -f "$stop_file"
    fi
}

# Run main with all arguments
main "$@"

#═══════════════════════════════════════════════════════════════════════════════
# AUTO-RESTART WRAPPER
#═══════════════════════════════════════════════════════════════════════════════
# To run with auto-restart on crash, use:
#   ./mine.sh --loop
# This will restart mining automatically if the script crashes.
# To stop: touch /tmp/opensy_stop_mining
#═══════════════════════════════════════════════════════════════════════════════

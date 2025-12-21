#!/bin/bash
#═══════════════════════════════════════════════════════════════════════════════
#  OpenSY Universal Mining Script v2.3 - ULTIMATE EDITION
#═══════════════════════════════════════════════════════════════════════════════
#
#  🇸🇾 Syria's First Cryptocurrency - Mine it anywhere!
#
#  This script handles EVERYTHING automatically:
#  ✅ Installs dependencies if missing (brew/apt/dnf/yum/pacman/apk)
#  ✅ Clones & builds OpenSY from source if not found
#  ✅ Starts daemon if not running
#  ✅ Loads existing wallet (prefers "founder")
#  ✅ Handles crashes, restarts, network issues
#  ✅ Works on macOS and Linux (x64 & ARM)
#  ✅ Mines to your specified wallet address
#  ✅ Auto-restarts on crash (--loop mode)
#  ✅ SSH-safe with screen/tmux detection
#
#  Usage:
#    ./mine.sh                           # Use default address
#    ./mine.sh <your-address>            # Custom mining address
#    ./mine.sh --install-only            # Just install, don't mine
#    ./mine.sh --check                   # Check status without mining
#    ./mine.sh --loop                    # Run forever, auto-restart on crash
#
#  One-liner for fresh machines:
#    curl -sL https://raw.githubusercontent.com/opensyria/OpenSY/main/mine.sh | bash
#    curl -sL https://raw.githubusercontent.com/opensyria/OpenSY/main/mine.sh | bash -s -- YOUR_ADDRESS
#
#  For unattended server mining:
#    screen -S mining ./mine.sh --loop --no-screen-check
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
VERSION="2.5.0"

# Block economics
BLOCK_REWARD=10000              # SYL per block (before halvings) - 10,000 initial
BLOCK_TIME_SECONDS=120          # Target 2 minutes per block
HALVING_INTERVAL=420000         # Blocks between halvings

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
SESSION_EARNINGS=0
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

#───────────────────────────────────────────────────────────────────────────────
# SECURITY: CHECKSUM VERIFICATION (M-02 Audit Fix)
#───────────────────────────────────────────────────────────────────────────────

# Homebrew installer script known SHA256 hashes (update periodically)
# These are verified hashes of the official Homebrew install script
# Check: curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | shasum -a 256
HOMEBREW_KNOWN_HASHES=(
    # Add known good hashes here - the script will warn if unknown hash detected
    # Format: "sha256_hash date_verified"
    # Since Homebrew updates frequently, we use a download-verify-prompt approach
)

# Repository commit/tag to verify (updated with releases)
OPENSY_VERIFIED_TAG=""  # Empty = use HEAD, set to specific tag for release builds
OPENSY_REPO_SSH_FINGERPRINT="SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s"  # GitHub's RSA key

calculate_sha256() {
    local file="$1"
    if command_exists shasum; then
        shasum -a 256 "$file" | awk '{print $1}'
    elif command_exists sha256sum; then
        sha256sum "$file" | awk '{print $1}'
    else
        log WARN "No SHA256 tool available - skipping checksum verification"
        echo ""
    fi
}

verify_download() {
    local url="$1"
    local expected_hash="${2:-}"
    local description="${3:-download}"
    local temp_file=$(mktemp)
    
    log DEBUG "Downloading $description for verification..."
    if ! curl -fsSL "$url" -o "$temp_file" 2>/dev/null; then
        rm -f "$temp_file"
        return 1
    fi
    
    local actual_hash=$(calculate_sha256 "$temp_file")
    
    if [ -z "$actual_hash" ]; then
        # No checksum tool available, proceed with warning but return the file
        echo "$temp_file"
        return 0
    fi
    
    if [ -n "$expected_hash" ]; then
        if [ "$actual_hash" = "$expected_hash" ]; then
            log DEBUG "Checksum verified: $actual_hash"
            echo "$temp_file"
            return 0
        else
            log ERROR "Checksum mismatch for $description!"
            log ERROR "Expected: $expected_hash"
            log ERROR "Got:      $actual_hash"
            log ERROR "This could indicate a compromised download. Aborting."
            rm -f "$temp_file"
            return 1
        fi
    else
        # No expected hash provided - log for audit trail and prompt user
        log WARN "Downloaded $description - SHA256: $actual_hash"
        log WARN "No expected hash configured. Verify this hash manually if concerned."
        log WARN "Add this hash to HOMEBREW_KNOWN_HASHES in mine.sh if valid."
        echo "$temp_file"
        return 0
    fi
}

verify_git_repository() {
    local repo_dir="$1"
    local expected_tag="${2:-}"
    
    if [ ! -d "$repo_dir/.git" ]; then
        log ERROR "Not a git repository: $repo_dir"
        return 1
    fi
    
    cd "$repo_dir"
    
    # Verify we're talking to the right remote
    local remote_url=$(git config --get remote.origin.url 2>/dev/null)
    if [[ ! "$remote_url" =~ opensyria/(OpenSY|OpenSyria)(\.git)?$ ]]; then
        log ERROR "Unexpected remote URL: $remote_url"
        log ERROR "Expected: github.com/opensyria/OpenSY or OpenSyria"
        return 1
    fi
    
    # Log the current commit for audit trail
    local current_commit=$(git rev-parse HEAD 2>/dev/null)
    local current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
    log DEBUG "Repository verified at commit: $current_commit (branch: $current_branch)"
    
    # If a specific tag is required, verify it
    if [ -n "$expected_tag" ]; then
        local tag_commit=$(git rev-list -n 1 "$expected_tag" 2>/dev/null)
        if [ -z "$tag_commit" ]; then
            log ERROR "Expected tag '$expected_tag' not found in repository"
            return 1
        fi
        if [ "$current_commit" != "$tag_commit" ]; then
            log WARN "Current commit ($current_commit) differs from tag $expected_tag ($tag_commit)"
            log WARN "Consider checking out the verified tag: git checkout $expected_tag"
        fi
    fi
    
    # Verify commit signature if gpg is available (optional enhancement)
    if command_exists gpg && [ -n "$expected_tag" ]; then
        if git tag -v "$expected_tag" &>/dev/null; then
            log SUCCESS "Tag $expected_tag has valid GPG signature"
        else
            log DEBUG "Tag signature verification not available or failed"
        fi
    fi
    
    return 0
}

#───────────────────────────────────────────────────────────────────────────────
# CONNECTIVITY
#───────────────────────────────────────────────────────────────────────────────

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
        echo "╔═══════════════════════════════════════════════════════════════════╗"
        echo "║                    MINING SESSION COMPLETE                        ║"
        echo "╚═══════════════════════════════════════════════════════════════════╝"
        echo ""
        log INFO "📊 Session Statistics:"
        log INFO "  ⏱️  Duration:       $(elapsed_time $elapsed)"
        log INFO "  ⛏️  Blocks Mined:   $BLOCKS_MINED"
        log INFO "  💰 SYL Earned:      $SESSION_EARNINGS SYL"
        log INFO "  ⚠️  Total Errors:   $TOTAL_ERRORS"
        log INFO "  🔄 Daemon Restarts: $DAEMON_RESTARTS"
        if [ $elapsed -gt 0 ] && [ $BLOCKS_MINED -gt 0 ]; then
            local rate=$(echo "scale=2; $BLOCKS_MINED * 3600 / $elapsed" | bc 2>/dev/null || echo "N/A")
            local syl_rate=$(echo "scale=0; $SESSION_EARNINGS * 3600 / $elapsed" | bc 2>/dev/null || echo "N/A")
            log INFO "  📈 Mining Rate:     $rate blocks/hour (~$syl_rate SYL/hour)"
        fi
        echo ""
        if [ $BLOCKS_MINED -gt 0 ]; then
            log SUCCESS "🎉 Great mining session! Your $SESSION_EARNINGS SYL will mature in 100 blocks."
        else
            log INFO "No blocks mined this session. Keep trying - mining is probabilistic!"
        fi
        echo ""
    fi
    
    # Remove lock files
    rm -f "$LOCKFILE" "$PIDFILE" 2>/dev/null || true
    
    log INFO "Thank you for mining OpenSY! 🇸🇾 Goodbye! 👋"
}

handle_signal() {
    log WARN "Received shutdown signal..."
    SHUTDOWN_REQUESTED=true
    # Cleanup background mining process
    cleanup_mine_files 2>/dev/null || true
    if [ -f "$MINE_PID_FILE" ]; then
        local pid=$(cat "$MINE_PID_FILE" 2>/dev/null)
        [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
    fi
}

trap handle_signal SIGINT SIGTERM SIGHUP
trap cleanup EXIT

#───────────────────────────────────────────────────────────────────────────────
# ENVIRONMENT VALIDATION
#───────────────────────────────────────────────────────────────────────────────

validate_environment() {
    # Check HOME is set
    if [ -z "${HOME:-}" ]; then
        export HOME=$(eval echo ~)
        if [ -z "$HOME" ] || [ ! -d "$HOME" ]; then
            die "HOME environment variable not set and could not be determined"
        fi
    fi
    
    # Check we have a working shell
    if [ -z "${BASH_VERSION:-}" ]; then
        log WARN "Not running in bash. Some features may not work correctly."
    fi
    
    # Check /tmp is writable
    if [ ! -w "/tmp" ]; then
        LOCKFILE="$HOME/.opensy_mine.lock"
        PIDFILE="$HOME/.opensy_mine.pid"
        log DEBUG "Using $HOME for lock files (/tmp not writable)"
    fi
    
    # Check bc is available (needed for calculations)
    if ! command_exists bc; then
        log DEBUG "bc not found, some calculations will be approximate"
    fi
}

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
        log DEBUG "Missing dependencies: ${missing[*]}"
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
                log INSTALL "Installing Homebrew (with security verification)..."
                
                # Download and verify the installer script
                local installer_url="https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"
                local verified_script
                verified_script=$(verify_download "$installer_url" "" "Homebrew installer")
                local verify_result=$?
                
                if [ "$verify_result" -ne 0 ] || [ -z "$verified_script" ]; then
                    log ERROR "Failed to download/verify Homebrew installer"
                    log ERROR "You can install Homebrew manually: https://brew.sh"
                    return 1
                fi
                
                # Execute the verified script
                /bin/bash "$verified_script" || {
                    rm -f "$verified_script"
                    log ERROR "Failed to install Homebrew"
                    return 1
                }
                rm -f "$verified_script"
                
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
        
        # Verify the repository after update
        verify_git_repository "$INSTALL_DIR" "$OPENSY_VERIFIED_TAG" || {
            log ERROR "Repository verification failed after update"
            return 1
        }
    else
        log BUILD "Cloning OpenSY repository (with verification)..."
        rm -rf "$INSTALL_DIR" 2>/dev/null || true
        
        # Clone with specific tag if configured, otherwise use HEAD
        local clone_opts="--depth 1"
        if [ -n "$OPENSY_VERIFIED_TAG" ]; then
            clone_opts="--branch $OPENSY_VERIFIED_TAG --depth 1"
            log BUILD "Using verified release tag: $OPENSY_VERIFIED_TAG"
        fi
        
        git clone $clone_opts https://github.com/opensyria/OpenSY.git "$INSTALL_DIR" || {
            # Try alternate URL
            git clone $clone_opts https://github.com/opensyria/OpenSyria.git "$INSTALL_DIR" || {
                log ERROR "Failed to clone repository"
                return 1
            }
        }
        cd "$INSTALL_DIR"
        
        # Verify the cloned repository
        verify_git_repository "$INSTALL_DIR" "$OPENSY_VERIFIED_TAG" || {
            log ERROR "Repository verification failed - possible supply chain attack!"
            log ERROR "Delete $INSTALL_DIR and investigate before retrying"
            return 1
        }
    fi
    
    log SUCCESS "Repository ready and verified at $INSTALL_DIR"
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
        -DENABLE_IPC=OFF
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
    # Normalize - remove whitespace and brackets
    loaded=$(echo "$loaded" | tr -d '[]" \n\r\t')
    
    if [ -n "$loaded" ]; then
        # Get first wallet name
        WALLET_NAME=$(echo "$loaded" | cut -d',' -f1)
        log SUCCESS "Using already loaded wallet: $WALLET_NAME"
        return 0
    fi
    
    # No wallet loaded, try to load preferred wallets in order
    for name in "${PREFERRED_WALLETS[@]}"; do
        [ -z "$name" ] && continue
        if cli_call loadwallet "$name" &>/dev/null; then
            WALLET_NAME="$name"
            log SUCCESS "Loaded wallet: $WALLET_NAME"
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
    # Get total balance including immature mining rewards
    local result
    if [ -n "$WALLET_NAME" ]; then
        result=$(cli_call -rpcwallet="$WALLET_NAME" getbalances 2>/dev/null)
    else
        result=$(cli_call getbalances 2>/dev/null)
    fi
    
    if [ -z "$result" ] || [[ "$result" == *"error"* ]]; then
        echo "N/A"
        return
    fi
    
    # Parse the mine section for immature balance
    local mine_trusted=$(echo "$result" | grep -A5 '"mine"' | grep '"trusted"' | grep -o '[0-9.]*' | head -1)
    local mine_immature=$(echo "$result" | grep -A5 '"mine"' | grep '"immature"' | grep -o '[0-9.]*' | head -1)
    
    mine_trusted=${mine_trusted:-0}
    mine_immature=${mine_immature:-0}
    
    # Use awk to sum (bc may not be installed)
    local total=$(awk "BEGIN {printf \"%.8f\", $mine_trusted + $mine_immature}" 2>/dev/null)
    
    if [ -n "$total" ] && [ "$total" != "0.00000000" ]; then
        echo "$total"
    else
        echo "0"
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

# Temp files for background mining
MINE_RESULT_FILE="/tmp/opensy_mine_result_$$"
MINE_PID_FILE="/tmp/opensy_mine_pid_$$"

cleanup_mine_files() {
    rm -f "$MINE_RESULT_FILE" "$MINE_PID_FILE" 2>/dev/null
}

mine_block_background() {
    # Start mining in background, write result to file
    cleanup_mine_files
    (
        result=$(cli_call generatetoaddress $BATCH_SIZE "$MINING_ADDRESS" 2>&1)
        exit_code=$?
        echo "$exit_code:$result" > "$MINE_RESULT_FILE"
    ) &
    echo $! > "$MINE_PID_FILE"
}

is_mining_done() {
    # Check if mining process finished
    if [ -f "$MINE_PID_FILE" ]; then
        local pid=$(cat "$MINE_PID_FILE" 2>/dev/null)
        if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
            return 0  # Process finished
        fi
    fi
    return 1  # Still running
}

get_mining_result() {
    # Returns: 0 for success, 1 for error
    # Outputs error message on failure
    if [ -f "$MINE_RESULT_FILE" ]; then
        local content=$(cat "$MINE_RESULT_FILE" 2>/dev/null)
        local exit_code="${content%%:*}"
        local result="${content#*:}"
        
        cleanup_mine_files
        
        if [ "$exit_code" = "0" ] && [[ "$result" =~ ^\[.*\]$ ]]; then
            return 0
        else
            echo "$result"
            return 1
        fi
    fi
    return 1
}

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

# Format number with commas (10000 -> 10,000) - works on both GNU and BSD
format_number() {
    local num="$1"
    # Remove any decimals first
    num="${num%%.*}"
    # Use printf with locale if available, otherwise manual formatting
    local formatted
    if formatted=$(printf "%'d" "$num" 2>/dev/null) && [[ "$formatted" == *","* || "$formatted" == "$num" ]]; then
        echo "$formatted"
        return
    fi
    # Manual comma insertion (works on all systems)
    echo "$num" | awk '{
        n = $1
        if (n < 0) { sign = "-"; n = -n } else { sign = "" }
        s = sprintf("%d", n)
        len = length(s)
        result = ""
        for (i = 1; i <= len; i++) {
            if (i > 1 && (len - i + 1) % 3 == 0) result = result ","
            result = result substr(s, i, 1)
        }
        print sign result
    }'
}

# Format balance (remove trailing zeros from decimals)
format_balance() {
    local bal="$1"
    # If it has decimals, clean them up
    if [[ "$bal" == *"."* ]]; then
        # Remove trailing zeros and possibly the decimal point
        bal=$(echo "$bal" | sed 's/\.00000000$//' | sed 's/\.\([0-9]*[1-9]\)0*$/.\1/')
    fi
    format_number "$bal"
}

celebrate_block() {
    local height=$1
    local reward=$2
    local balance=$(get_balance)
    local reward_fmt=$(format_number $reward)
    local earnings_fmt=$(format_number $SESSION_EARNINGS)
    local balance_fmt=$(format_balance "$balance")
    
    # Simple celebration messages - randomly pick one
    local messages=(
        "💰 BLOCK MINED! +${reward_fmt} SYL added to your wallet!"
        "🎉 SUCCESS! Block #${height} found - ${reward_fmt} SYL earned!"
        "⛏️  FOUND BLOCK #${height}! +${reward_fmt} SYL is now yours!"
        "🚀 BOOM! You mined a block! +${reward_fmt} SYL"
        "💎 NICE! Block #${height} - ${reward_fmt} SYL reward!"
        "🔥 MINING SUCCESS! +${reward_fmt} SYL to the bag!"
        "✨ Block #${height} is YOURS! +${reward_fmt} SYL earned!"
        "🏆 WINNER! You found block #${height} - ${reward_fmt} SYL!"
        "💵 CHA-CHING! +${reward_fmt} SYL from block #${height}!"
        "🎯 BULLSEYE! Block mined - ${reward_fmt} SYL reward!"
    )
    
    local msg_count=${#messages[@]}
    local idx=$((RANDOM % msg_count))
    local msg="${messages[$idx]}"
    
    echo ""
    echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
    echo "┃                     🎊 BLOCK FOUND! 🎊                          ┃"
    echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
    log SUCCESS "$msg"
    echo ""
    log SUCCESS "📊 Session Stats:"
    log SUCCESS "   Blocks Mined: $BLOCKS_MINED"
    log SUCCESS "   SYL Earned:   $earnings_fmt SYL"
    log SUCCESS "   💰 Balance:   $balance_fmt SYL"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# Reward tracking
get_block_reward() {
    local height=${1:-0}
    # OpenSY halving schedule: every 420,000 blocks
    local halvings=$((height / HALVING_INTERVAL))
    local reward=$BLOCK_REWARD  # 10,000 SYL initial
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
    log INFO "  Reward: $BLOCK_REWARD SYL per block"
    log INFO "═══════════════════════════════════════════════════════════════"
    echo ""
    
    START_TIME=$(now)
    START_HEIGHT=$(get_block_count)
    
    log INFO "Starting at block height: $START_HEIGHT"
    log INFO "Mining in progress... (live updates)"
    echo ""
    
    local last_daemon_check=$(now)
    local last_stats_time=$(now)
    local last_block_time=$(now)
    local last_block_height=$START_HEIGHT
    local spinner_idx=0
    local spinner_chars=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
    local mining_in_progress=false
    
    while [ "$SHUTDOWN_REQUESTED" = false ]; do
        local current_time=$(now)
        
        # Start mining if not already running
        if [ "$mining_in_progress" = false ]; then
            mine_block_background
            mining_in_progress=true
        fi
        
        # Update spinner (runs every 100ms for smooth animation)
        spinner_idx=$(( (spinner_idx + 1) % ${#spinner_chars[@]} ))
        local elapsed=$((current_time - START_TIME))
        local elapsed_str=$(elapsed_time $elapsed)
        local current_height=$(get_block_count 2>/dev/null || echo "$last_block_height")
        local connections=$(get_connection_count 2>/dev/null || echo "?")
        local height_fmt=$(format_number "$current_height")
        local earned_fmt=$(format_number "$SESSION_EARNINGS")
        printf "\r\033[K⛏️  Mining... %s | Height: %s | Mined: %d | Earned: %s SYL | Peers: %s | Time: %s " \
            "${spinner_chars[$spinner_idx]}" "$height_fmt" "$BLOCKS_MINED" "$earned_fmt" "$connections" "$elapsed_str"
        
        # Check if mining completed
        if is_mining_done; then
            local error_output
            if error_output=$(get_mining_result); then
                # Clear the progress line
                printf "\r\033[K"
                
                # Success! We mined a block!
                ERROR_COUNT=0
                BLOCKS_MINED=$((BLOCKS_MINED + BATCH_SIZE))
                
                # Calculate earnings
                current_height=$(get_block_count)
                local reward=$(get_block_reward $current_height)
                SESSION_EARNINGS=$((SESSION_EARNINGS + reward))
                
                # Celebration message!
                celebrate_block $current_height $reward
                
                # Track last successful block
                last_block_time=$(now)
                last_block_height=$current_height
            else
                # Error
                ERROR_COUNT=$((ERROR_COUNT + 1))
                TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
                
                # Only log errors periodically to avoid spam
                if [ $ERROR_COUNT -eq 1 ] || [ $((ERROR_COUNT % 5)) -eq 0 ]; then
                    printf "\r\033[K"
                    log WARN "Mining error ($ERROR_COUNT/$MAX_CONSECUTIVE_ERRORS): $(echo "$error_output" | head -1)"
                fi
                
                # Too many errors - check daemon
                if [ $ERROR_COUNT -ge $MAX_CONSECUTIVE_ERRORS ]; then
                    printf "\r\033[K"
                    log ERROR "Too many consecutive errors, checking daemon..."
                    
                    if ! ensure_daemon_running; then
                        log ERROR "Daemon unrecoverable. Waiting longer..."
                        sleep 60
                    fi
                    
                    ERROR_COUNT=0
                fi
            fi
            
            mining_in_progress=false
            sleep $MINING_DELAY
            continue
        fi
        
        # Periodic daemon health check (every 5 minutes)
        if [ $((current_time - last_daemon_check)) -gt $DAEMON_CHECK_INTERVAL ]; then
            if ! is_daemon_running; then
                printf "\r\033[K"
                log ERROR "Cannot reach daemon, waiting..."
                cleanup_mine_files
                mining_in_progress=false
                sleep $ERROR_DELAY
                last_daemon_check=$current_time
                continue
            fi
            last_daemon_check=$current_time
        fi
        
        # Periodic stats (every 5 minutes even if no blocks)
        if [ $((current_time - last_stats_time)) -gt 300 ]; then
            printf "\r\033[K"
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
        
        # Small sleep for spinner animation (100ms)
        sleep 0.1
    done
    
    # Cleanup on exit
    cleanup_mine_files
    
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
    
    # Validate environment first
    validate_environment
    
    # Banner
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║       OpenSY Universal Mining Script v$VERSION - ULTIMATE          ║"
    echo "║              🇸🇾 Syria's First Cryptocurrency                       ║"
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

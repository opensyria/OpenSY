#!/bin/bash
# =============================================================================
# OpenSY Node Setup Script
# =============================================================================
# This script automates the initial setup of an OpenSY seed node.
# Supports: Ubuntu 22.04+ (AMD64 and ARM64)
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/opensy/OpenSY/main/contrib/deploy/setup-node.sh | bash
#
# Or download and run:
#   wget https://raw.githubusercontent.com/opensy/OpenSY/main/contrib/deploy/setup-node.sh
#   chmod +x setup-node.sh
#   sudo ./setup-node.sh
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
OPENSY_USER="opensy"
OPENSY_HOME="/home/${OPENSY_USER}"
OPENSY_DATA="${OPENSY_HOME}/.opensy"
OPENSY_SRC="/opt/opensy"
OPENSY_REPO="https://github.com/opensyria/OpenSY.git"
OPENSY_BRANCH="main"

# Default ports
P2P_PORT=9633
RPC_PORT=9632

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

detect_architecture() {
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            ARCH_NAME="amd64"
            ;;
        aarch64)
            ARCH_NAME="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    log_info "Detected architecture: $ARCH_NAME"
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS_NAME=$ID
        OS_VERSION=$VERSION_ID
    else
        log_error "Cannot detect OS. /etc/os-release not found."
        exit 1
    fi
    
    if [[ "$OS_NAME" != "ubuntu" ]] && [[ "$OS_NAME" != "debian" ]]; then
        log_warn "This script is tested on Ubuntu/Debian. Proceeding anyway..."
    fi
    
    log_info "Detected OS: $OS_NAME $OS_VERSION"
}

# =============================================================================
# Installation Functions
# =============================================================================

install_dependencies() {
    log_info "Installing system dependencies..."
    
    apt-get update
    apt-get upgrade -y
    
    apt-get install -y \
        build-essential \
        cmake \
        pkg-config \
        libevent-dev \
        libboost-dev \
        libboost-system-dev \
        libboost-filesystem-dev \
        libboost-test-dev \
        libsqlite3-dev \
        libzmq3-dev \
        libssl-dev \
        git \
        ufw \
        fail2ban \
        htop \
        tmux \
        jq \
        curl \
        wget \
        unzip
    
    log_success "Dependencies installed successfully"
}

create_user() {
    log_info "Creating opensy user..."
    
    if id "$OPENSY_USER" &>/dev/null; then
        log_warn "User $OPENSY_USER already exists"
    else
        useradd -m -s /bin/bash "$OPENSY_USER"
        log_success "User $OPENSY_USER created"
    fi
    
    # Create directories
    mkdir -p "$OPENSY_SRC"
    mkdir -p "$OPENSY_DATA"
    chown -R "$OPENSY_USER:$OPENSY_USER" "$OPENSY_SRC"
    chown -R "$OPENSY_USER:$OPENSY_USER" "$OPENSY_DATA"
}

configure_firewall() {
    log_info "Configuring firewall..."
    
    ufw default deny incoming
    ufw default allow outgoing
    
    # SSH
    ufw allow 22/tcp comment 'SSH'
    
    # OpenSY P2P
    ufw allow ${P2P_PORT}/tcp comment 'OpenSY P2P'
    
    # Enable firewall
    echo "y" | ufw enable
    
    log_success "Firewall configured"
}

configure_fail2ban() {
    log_info "Configuring fail2ban..."
    
    cat > /etc/fail2ban/jail.local << EOF
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
EOF
    
    systemctl enable fail2ban
    systemctl restart fail2ban
    
    log_success "fail2ban configured"
}

clone_repository() {
    log_info "Cloning OpenSY repository..."
    
    if [[ -d "${OPENSY_SRC}/source" ]]; then
        log_warn "Repository already exists, pulling latest changes..."
        cd "${OPENSY_SRC}/source"
        sudo -u "$OPENSY_USER" git pull origin "$OPENSY_BRANCH"
    else
        sudo -u "$OPENSY_USER" git clone "$OPENSY_REPO" "${OPENSY_SRC}/source"
        cd "${OPENSY_SRC}/source"
        sudo -u "$OPENSY_USER" git checkout "$OPENSY_BRANCH"
    fi
    
    log_success "Repository cloned/updated"
}

build_opensy() {
    log_info "Building OpenSY (this may take 10-30 minutes)..."
    
    cd "${OPENSY_SRC}/source"
    
    # Configure build
    sudo -u "$OPENSY_USER" cmake -B build \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_GUI=OFF \
        -DBUILD_TESTS=OFF
    
    # Build with all available cores
    CORES=$(nproc)
    log_info "Building with $CORES cores..."
    sudo -u "$OPENSY_USER" cmake --build build -j"$CORES"
    
    # Verify build
    if [[ -f "${OPENSY_SRC}/source/build/bin/opensyd" ]]; then
        log_success "Build completed successfully"
        "${OPENSY_SRC}/source/build/bin/opensyd" --version
    else
        log_error "Build failed - opensyd binary not found"
        exit 1
    fi
}

generate_config() {
    log_info "Generating configuration..."
    
    # Generate secure RPC credentials
    RPC_USER="opensyrpc"
    RPC_PASS=$(openssl rand -hex 32)
    
    # Prompt for node name
    read -p "Enter node name (e.g., seed1, seed2): " NODE_NAME
    NODE_NAME=${NODE_NAME:-"node1"}
    
    # Create configuration file
    cat > "${OPENSY_DATA}/opensy.conf" << EOF
# =============================================================================
# OpenSY Node Configuration
# Node: ${NODE_NAME}.opensy.net
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
# =============================================================================

# ------------------------------------------------------------------------------
# Network Settings
# ------------------------------------------------------------------------------
server=1
daemon=1
listen=1
port=${P2P_PORT}
bind=0.0.0.0

# Maximum peer connections
maxconnections=125

# Daily upload limit (MB) - adjust based on bandwidth
maxuploadtarget=5000

# ------------------------------------------------------------------------------
# RPC Settings
# ------------------------------------------------------------------------------
rpcuser=${RPC_USER}
rpcpassword=${RPC_PASS}
rpcbind=127.0.0.1
rpcallowip=127.0.0.1
rpcport=${RPC_PORT}

# ------------------------------------------------------------------------------
# Indexing
# ------------------------------------------------------------------------------
# Enable transaction index (required for block explorer)
txindex=1

# ------------------------------------------------------------------------------
# Logging
# ------------------------------------------------------------------------------
debug=net
logips=1
logtimestamps=1
shrinkdebugfile=1

# ------------------------------------------------------------------------------
# Performance
# ------------------------------------------------------------------------------
# Database cache size (MB) - adjust based on available RAM
# Recommended: 25% of available RAM
dbcache=512

# Parallel script verification threads
par=2

# ------------------------------------------------------------------------------
# Seed Nodes
# ------------------------------------------------------------------------------
# Connect to official seed nodes
seednode=seed.opensyria.net:9633
seednode=seed2.opensyria.net:9633
seednode=seed3.opensyria.net:9633

# Add manual seed node IPs if DNS seeds are not yet active
# addnode=<IP>:9633
EOF
    
    chown "$OPENSY_USER:$OPENSY_USER" "${OPENSY_DATA}/opensy.conf"
    chmod 600 "${OPENSY_DATA}/opensy.conf"
    
    # Save credentials separately
    cat > "${OPENSY_DATA}/.rpc_credentials" << EOF
RPC_USER=${RPC_USER}
RPC_PASS=${RPC_PASS}
EOF
    chown "$OPENSY_USER:$OPENSY_USER" "${OPENSY_DATA}/.rpc_credentials"
    chmod 600 "${OPENSY_DATA}/.rpc_credentials"
    
    log_success "Configuration generated"
    log_info "RPC credentials saved to ${OPENSY_DATA}/.rpc_credentials"
}

create_systemd_service() {
    log_info "Creating systemd service..."
    
    cat > /etc/systemd/system/opensyd.service << EOF
[Unit]
Description=OpenSY daemon
Documentation=https://opensy.net/
After=network-online.target
Wants=network-online.target

[Service]
Type=forking
User=${OPENSY_USER}
Group=${OPENSY_USER}

Environment="HOME=${OPENSY_HOME}"

ExecStart=${OPENSY_SRC}/source/build/bin/opensyd \\
    -daemon \\
    -pid=${OPENSY_DATA}/opensyd.pid \\
    -conf=${OPENSY_DATA}/opensy.conf \\
    -datadir=${OPENSY_DATA}

ExecStop=${OPENSY_SRC}/source/build/bin/opensy-cli \\
    -conf=${OPENSY_DATA}/opensy.conf \\
    -datadir=${OPENSY_DATA} \\
    stop

PIDFile=${OPENSY_DATA}/opensyd.pid

Restart=on-failure
RestartSec=30
TimeoutStartSec=infinity
TimeoutStopSec=600

# Hardening measures
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true
PrivateDevices=true
MemoryDenyWriteExecute=true

# Resource limits
LimitNOFILE=64000

[Install]
WantedBy=multi-user.target
EOF
    
    # Create helper script
    cat > /usr/local/bin/opensy-cli << 'EOF'
#!/bin/bash
exec /opt/opensy/source/build/bin/opensy-cli \
    -conf=/home/opensy/.opensy/opensy.conf \
    -datadir=/home/opensy/.opensy \
    "$@"
EOF
    chmod +x /usr/local/bin/opensy-cli
    
    systemctl daemon-reload
    systemctl enable opensyd
    
    log_success "Systemd service created"
}

start_node() {
    log_info "Starting OpenSY node..."
    
    systemctl start opensyd
    
    # Wait for node to start
    sleep 5
    
    if systemctl is-active --quiet opensyd; then
        log_success "OpenSY node started successfully"
    else
        log_error "Failed to start OpenSY node"
        journalctl -u opensyd --no-pager -n 50
        exit 1
    fi
}

print_summary() {
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "Unable to detect")
    
    echo ""
    echo "============================================================================="
    echo -e "${GREEN}OpenSY Node Setup Complete!${NC}"
    echo "============================================================================="
    echo ""
    echo "Node Information:"
    echo "  - Public IP: ${PUBLIC_IP}"
    echo "  - P2P Port: ${P2P_PORT}"
    echo "  - RPC Port: ${RPC_PORT} (localhost only)"
    echo "  - Data Directory: ${OPENSY_DATA}"
    echo ""
    echo "Useful Commands:"
    echo "  - Check status:     systemctl status opensyd"
    echo "  - View logs:        journalctl -u opensyd -f"
    echo "  - Blockchain info:  opensy-cli getblockchaininfo"
    echo "  - Peer info:        opensy-cli getpeerinfo"
    echo "  - Network info:     opensy-cli getnetworkinfo"
    echo "  - Stop node:        systemctl stop opensyd"
    echo "  - Restart node:     systemctl restart opensyd"
    echo ""
    echo "RPC Credentials:"
    echo "  - Stored in: ${OPENSY_DATA}/.rpc_credentials"
    echo ""
    echo "Next Steps:"
    echo "  1. Wait for blockchain sync to complete"
    echo "  2. Add this node's IP to DNS seeds"
    echo "  3. Monitor with: journalctl -u opensyd -f"
    echo ""
    echo "============================================================================="
}

# =============================================================================
# Main Script
# =============================================================================

main() {
    echo ""
    echo "============================================================================="
    echo "  OpenSY Node Setup Script"
    echo "  Version: 1.0"
    echo "============================================================================="
    echo ""
    
    check_root
    detect_os
    detect_architecture
    
    echo ""
    read -p "This will install OpenSY node on this system. Continue? [y/N] " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Installation cancelled"
        exit 0
    fi
    
    install_dependencies
    create_user
    configure_firewall
    configure_fail2ban
    clone_repository
    build_opensy
    generate_config
    create_systemd_service
    start_node
    print_summary
}

# Run main function
main "$@"

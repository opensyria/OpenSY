#!/bin/bash
# =============================================================================
# OpenSY DNS Seeder Setup Script
# =============================================================================
# Sets up the DNS seeder service for OpenSY network discovery.
# Should be run on a server that already has OpenSY node installed.
#
# Prerequisites:
#   - OpenSY node running (setup-node.sh completed)
#   - Port 53 open (UDP and TCP)
#   - Domain configured to delegate DNS to this server
#
# Usage:
#   sudo ./setup-dns-seeder.sh
# =============================================================================

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SEEDER_DIR="/opt/opensy/seeder"
OPENSY_USER="opensy"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if OpenSY is installed
    if ! systemctl is-active --quiet opensyd; then
        log_error "OpenSY node is not running. Please run setup-node.sh first."
        exit 1
    fi
    
    # Check port 53
    if netstat -tuln | grep -q ':53 '; then
        log_warn "Port 53 is already in use. You may need to stop systemd-resolved."
        echo "Run: sudo systemctl stop systemd-resolved"
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    log_success "Prerequisites check passed"
}

disable_systemd_resolved() {
    log_info "Disabling systemd-resolved (conflicts with port 53)..."
    
    # Stop and disable systemd-resolved
    systemctl stop systemd-resolved 2>/dev/null || true
    systemctl disable systemd-resolved 2>/dev/null || true
    
    # Update resolv.conf to use public DNS
    rm -f /etc/resolv.conf
    cat > /etc/resolv.conf << EOF
nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 1.1.1.1
EOF
    
    log_success "systemd-resolved disabled"
}

install_dependencies() {
    log_info "Installing dependencies..."
    
    apt-get update
    apt-get install -y build-essential libssl-dev
    
    log_success "Dependencies installed"
}

clone_and_build_seeder() {
    log_info "Building OpenSY DNS seeder..."
    
    mkdir -p "$SEEDER_DIR"
    cd "$SEEDER_DIR"
    
    # Clone bitcoin-seeder
    if [[ -d "bitcoin-seeder" ]]; then
        cd bitcoin-seeder
        git pull origin master
    else
        git clone https://github.com/sipa/bitcoin-seeder.git
        cd bitcoin-seeder
    fi
    
    # Create OpenSY patch
    log_info "Applying OpenSY modifications..."
    
    # Backup original file
    cp main.cpp main.cpp.orig
    
    # Apply modifications using sed
    # Change message start bytes to OpenSY's SYLM
    sed -i 's/0xf9, 0xbe, 0xb4, 0xd9/0x53, 0x59, 0x4c, 0x4d/g' main.cpp
    
    # Change default port to 9633
    sed -i 's/8333/9633/g' main.cpp
    
    # Change default DNS host
    sed -i 's/seed\.bitcoin\.sipa\.be/seed.opensyria.net/g' main.cpp
    
    # Build
    make clean || true
    make
    
    if [[ -f "dnsseed" ]]; then
        log_success "DNS seeder built successfully"
    else
        log_error "Failed to build DNS seeder"
        exit 1
    fi
    
    chown -R "$OPENSY_USER:$OPENSY_USER" "$SEEDER_DIR"
}

configure_seeder() {
    log_info "Configuring DNS seeder..."
    
    # Prompt for configuration
    echo ""
    read -p "Enter DNS hostname (e.g., seed.opensyria.net): " DNS_HOST
    DNS_HOST=${DNS_HOST:-"seed.opensyria.net"}
    
    read -p "Enter nameserver hostname (e.g., ns1.opensyria.net): " NS_HOST
    NS_HOST=${NS_HOST:-"ns1.opensyria.net"}
    
    read -p "Enter admin email (e.g., admin@opensyria.net): " ADMIN_EMAIL
    ADMIN_EMAIL=${ADMIN_EMAIL:-"admin@opensyria.net"}
    
    # Save configuration
    cat > "${SEEDER_DIR}/seeder.conf" << EOF
DNS_HOST=${DNS_HOST}
NS_HOST=${NS_HOST}
ADMIN_EMAIL=${ADMIN_EMAIL}
EOF
    
    log_success "Configuration saved"
}

open_firewall_port() {
    log_info "Opening DNS port in firewall..."
    
    ufw allow 53/udp comment 'DNS Seeder UDP'
    ufw allow 53/tcp comment 'DNS Seeder TCP'
    
    log_success "Firewall updated"
}

create_systemd_service() {
    log_info "Creating systemd service..."
    
    # Load configuration
    source "${SEEDER_DIR}/seeder.conf"
    
    cat > /etc/systemd/system/opensy-seeder.service << EOF
[Unit]
Description=OpenSY DNS Seeder
Documentation=https://opensy.net/
After=network-online.target opensyd.service
Wants=network-online.target
Requires=opensyd.service

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${SEEDER_DIR}/bitcoin-seeder

ExecStart=${SEEDER_DIR}/bitcoin-seeder/dnsseed \\
    -h ${DNS_HOST} \\
    -n ${NS_HOST} \\
    -m ${ADMIN_EMAIL}

Restart=always
RestartSec=30

# DNS needs root for port 53
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable opensy-seeder
    
    log_success "Systemd service created"
}

start_seeder() {
    log_info "Starting DNS seeder..."
    
    systemctl start opensy-seeder
    
    sleep 3
    
    if systemctl is-active --quiet opensy-seeder; then
        log_success "DNS seeder started successfully"
    else
        log_error "Failed to start DNS seeder"
        journalctl -u opensy-seeder --no-pager -n 20
        exit 1
    fi
}

print_dns_instructions() {
    source "${SEEDER_DIR}/seeder.conf"
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "<YOUR_SERVER_IP>")
    
    echo ""
    echo "============================================================================="
    echo -e "${GREEN}OpenSY DNS Seeder Setup Complete!${NC}"
    echo "============================================================================="
    echo ""
    echo "DNS Configuration Required:"
    echo ""
    echo "Add these records to your domain's DNS (Cloudflare, Route53, etc.):"
    echo ""
    echo "┌─────────┬──────────────────┬───────────────────┬─────────┐"
    echo "│ Type    │ Name             │ Value             │ Proxy   │"
    echo "├─────────┼──────────────────┼───────────────────┼─────────┤"
    echo "│ A       │ ns1              │ ${PUBLIC_IP}      │ OFF     │"
    echo "│ NS      │ seed             │ ns1.opensyria.net │ -       │"
    echo "└─────────┴──────────────────┴───────────────────┴─────────┘"
    echo ""
    echo "How it works:"
    echo "  1. Users query seed.opensyria.net"
    echo "  2. DNS delegates to ns1.opensyria.net (this server)"
    echo "  3. This seeder responds with active OpenSY node IPs"
    echo ""
    echo "Useful Commands:"
    echo "  - Check status:   systemctl status opensy-seeder"
    echo "  - View logs:      journalctl -u opensy-seeder -f"
    echo "  - Test DNS:       dig seed.opensyria.net @${PUBLIC_IP}"
    echo ""
    echo "============================================================================="
}

# =============================================================================
# Main
# =============================================================================

main() {
    echo ""
    echo "============================================================================="
    echo "  OpenSY DNS Seeder Setup Script"
    echo "============================================================================="
    echo ""
    
    check_root
    check_prerequisites
    
    read -p "This will set up DNS seeder on this server. Continue? [y/N] " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Installation cancelled"
        exit 0
    fi
    
    disable_systemd_resolved
    install_dependencies
    clone_and_build_seeder
    configure_seeder
    open_firewall_port
    create_systemd_service
    start_seeder
    print_dns_instructions
}

main "$@"

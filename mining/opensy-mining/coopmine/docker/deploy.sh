#!/bin/bash
# CoopMine Docker deployment script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check requirements
check_requirements() {
    log_info "Checking requirements..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed"
        exit 1
    fi
    
    log_info "Requirements satisfied"
}

# Generate certificates if not exist
generate_certs() {
    if [ ! -d "certs" ] || [ ! -f "certs/server.crt" ]; then
        log_info "Generating TLS certificates..."
        mkdir -p certs
        
        # Use the gen_certs.sh script if available
        if [ -f "../../scripts/gen_certs.sh" ]; then
            cd ../..
            ./scripts/gen_certs.sh
            mv certs/* coopmine/docker/certs/
            cd "$SCRIPT_DIR"
        else
            log_warn "Certificate generation script not found. Please generate certificates manually."
        fi
    else
        log_info "Certificates already exist"
    fi
}

# Create config files
create_configs() {
    log_info "Setting up configuration files..."
    
    mkdir -p config
    
    if [ ! -f "config/coordinator.yaml" ]; then
        cp ../config/coordinator.example.yaml config/coordinator.yaml
        log_info "Created coordinator.yaml from example"
    fi
    
    if [ ! -f "config/worker.yaml" ]; then
        cp ../config/worker.example.yaml config/worker.yaml
        log_info "Created worker.yaml from example"
    fi
}

# Build images
build() {
    log_info "Building Docker images..."
    docker compose build
}

# Start cluster
start() {
    local workers=${1:-1}
    local with_monitoring=${2:-false}
    
    log_info "Starting CoopMine cluster with $workers worker(s)..."
    
    if [ "$with_monitoring" = true ]; then
        docker compose --profile monitoring up -d --scale worker=$workers
    else
        docker compose up -d --scale worker=$workers
    fi
    
    log_info "Cluster started!"
    log_info "Dashboard: http://localhost:8080"
    log_info "gRPC: localhost:50051"
    
    if [ "$with_monitoring" = true ]; then
        log_info "Prometheus: http://localhost:9091"
        log_info "Grafana: http://localhost:3000 (admin/admin)"
    fi
}

# Stop cluster
stop() {
    log_info "Stopping CoopMine cluster..."
    docker compose --profile monitoring down
}

# View logs
logs() {
    local service=${1:-}
    if [ -n "$service" ]; then
        docker compose logs -f "$service"
    else
        docker compose logs -f
    fi
}

# Show status
status() {
    docker compose ps
}

# Scale workers
scale() {
    local workers=${1:-1}
    log_info "Scaling to $workers worker(s)..."
    docker compose up -d --scale worker=$workers
}

# Clean up
clean() {
    log_info "Cleaning up..."
    docker compose --profile monitoring down -v
    rm -rf config certs
    log_info "Cleanup complete"
}

# Main
case "${1:-}" in
    build)
        check_requirements
        build
        ;;
    start)
        check_requirements
        generate_certs
        create_configs
        start "${2:-1}" "${3:-false}"
        ;;
    start-with-monitoring)
        check_requirements
        generate_certs
        create_configs
        start "${2:-1}" true
        ;;
    stop)
        stop
        ;;
    restart)
        stop
        start "${2:-1}"
        ;;
    logs)
        logs "${2:-}"
        ;;
    status)
        status
        ;;
    scale)
        scale "${2:-1}"
        ;;
    clean)
        clean
        ;;
    *)
        echo "CoopMine Docker Deployment"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  build                    Build Docker images"
        echo "  start [workers]          Start cluster with N workers (default: 1)"
        echo "  start-with-monitoring    Start with Prometheus & Grafana"
        echo "  stop                     Stop cluster"
        echo "  restart [workers]        Restart cluster"
        echo "  logs [service]           View logs"
        echo "  status                   Show container status"
        echo "  scale <workers>          Scale worker count"
        echo "  clean                    Remove all containers and data"
        echo ""
        echo "Examples:"
        echo "  $0 build"
        echo "  $0 start 3                # Start with 3 workers"
        echo "  $0 start-with-monitoring 2"
        echo "  $0 scale 5                # Scale to 5 workers"
        echo "  $0 logs coordinator"
        ;;
esac

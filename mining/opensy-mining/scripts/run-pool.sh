#!/bin/bash
# OpenSY Pool Server Quick Start Script
# Usage: ./run-pool.sh [start|stop|status]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$SCRIPT_DIR/../bin"
DOCKER_DIR="$SCRIPT_DIR/../docker"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

print_banner() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║         ⛏️  OpenSY Mining Pool  ⛏️             ║"
    echo "║       Professional Stratum Pool Server        ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_docker() {
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}Error: Docker is not installed${NC}"
        exit 1
    fi
    if ! docker info &> /dev/null; then
        echo -e "${RED}Error: Docker is not running${NC}"
        exit 1
    fi
}

check_binary() {
    if [ ! -f "$BIN_DIR/server" ]; then
        echo -e "${RED}Error: Pool server binary not found. Run 'make build' first.${NC}"
        exit 1
    fi
}

start_infrastructure() {
    echo -e "${YELLOW}Starting infrastructure (PostgreSQL + Redis)...${NC}"
    
    if [ -f "$DOCKER_DIR/docker-compose.yml" ]; then
        cd "$DOCKER_DIR"
        docker-compose up -d postgres redis
        echo -e "${GREEN}✓ Infrastructure started${NC}"
        
        # Wait for services to be ready
        echo -e "${YELLOW}Waiting for services to be ready...${NC}"
        sleep 5
        
        # Check services
        if docker-compose ps | grep -q "Up"; then
            echo -e "${GREEN}✓ PostgreSQL: Ready${NC}"
            echo -e "${GREEN}✓ Redis: Ready${NC}"
        else
            echo -e "${RED}Warning: Some services may not be ready${NC}"
        fi
    else
        echo -e "${RED}Error: docker-compose.yml not found${NC}"
        echo -e "${YELLOW}Starting with default ports (ensure PostgreSQL and Redis are running)${NC}"
    fi
}

stop_infrastructure() {
    echo -e "${YELLOW}Stopping infrastructure...${NC}"
    if [ -f "$DOCKER_DIR/docker-compose.yml" ]; then
        cd "$DOCKER_DIR"
        docker-compose down
        echo -e "${GREEN}✓ Infrastructure stopped${NC}"
    fi
}

run_pool() {
    echo -e "${GREEN}Starting OpenSY Mining Pool...${NC}"
    echo ""
    
    # Configuration with defaults
    STRATUM_ADDR="${STRATUM_ADDR:-0.0.0.0:3333}"
    METRICS_ADDR="${METRICS_ADDR:-0.0.0.0:8080}"
    NODE_URL="${NODE_URL:-http://localhost:9632}"
    NODE_USER="${NODE_USER:-}"
    NODE_PASS="${NODE_PASS:-}"
    DB_HOST="${DB_HOST:-localhost}"
    DB_PORT="${DB_PORT:-5432}"
    DB_USER="${DB_USER:-opensy}"
    DB_PASS="${DB_PASS:-opensy}"
    DB_NAME="${DB_NAME:-opensy_pool}"
    REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
    INIT_DIFF="${INIT_DIFF:-10000}"
    MIN_DIFF="${MIN_DIFF:-1000}"
    MAX_DIFF="${MAX_DIFF:-1000000}"
    
    echo -e "${YELLOW}Configuration:${NC}"
    echo "  Stratum:      $STRATUM_ADDR"
    echo "  Metrics:      $METRICS_ADDR"
    echo "  Node:         $NODE_URL"
    echo "  Database:     $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"
    echo "  Redis:        $REDIS_ADDR"
    echo "  Difficulty:   $INIT_DIFF (min: $MIN_DIFF, max: $MAX_DIFF)"
    echo ""
    
    exec "$BIN_DIR/server" \
        --stratum-addr="$STRATUM_ADDR" \
        --metrics-addr="$METRICS_ADDR" \
        --node-url="$NODE_URL" \
        --node-user="$NODE_USER" \
        --node-pass="$NODE_PASS" \
        --db-host="$DB_HOST" \
        --db-port="$DB_PORT" \
        --db-user="$DB_USER" \
        --db-password="$DB_PASS" \
        --db-name="$DB_NAME" \
        --redis-addr="$REDIS_ADDR" \
        --initial-difficulty="$INIT_DIFF" \
        --min-difficulty="$MIN_DIFF" \
        --max-difficulty="$MAX_DIFF" \
        --vardiff=true \
        --log-level=info
}

show_status() {
    echo -e "${YELLOW}=== Infrastructure Status ===${NC}"
    if [ -f "$DOCKER_DIR/docker-compose.yml" ]; then
        cd "$DOCKER_DIR"
        docker-compose ps
    fi
    echo ""
    echo -e "${YELLOW}=== Pool Endpoints ===${NC}"
    echo "  Stratum:  ${STRATUM_ADDR:-0.0.0.0:3333}"
    echo "  API:      http://${METRICS_ADDR:-localhost:8080}/api/stats"
    echo "  Metrics:  http://${METRICS_ADDR:-localhost:8080}/metrics"
    echo "  Health:   http://${METRICS_ADDR:-localhost:8080}/health"
}

show_help() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  start       Start infrastructure and pool server"
    echo "  infra       Start only infrastructure (PostgreSQL + Redis)"
    echo "  pool        Start only pool server (infra must be running)"
    echo "  stop        Stop infrastructure"
    echo "  status      Show status of services"
    echo "  help        Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  STRATUM_ADDR   Stratum listen address (default: 0.0.0.0:3333)"
    echo "  METRICS_ADDR   API/Metrics address (default: 0.0.0.0:8080)"
    echo "  NODE_URL       OpenSY node RPC URL (default: http://localhost:9632)"
    echo "  NODE_USER      Node RPC username"
    echo "  NODE_PASS      Node RPC password"
    echo "  DB_HOST        PostgreSQL host (default: localhost)"
    echo "  DB_PORT        PostgreSQL port (default: 5432)"
    echo "  DB_USER        PostgreSQL user (default: opensy)"
    echo "  DB_PASS        PostgreSQL password (default: opensy)"
    echo "  DB_NAME        PostgreSQL database (default: opensy_pool)"
    echo "  REDIS_ADDR     Redis address (default: localhost:6379)"
    echo "  INIT_DIFF      Initial difficulty (default: 10000)"
    echo ""
    echo "Examples:"
    echo "  # Quick start (starts everything)"
    echo "  $0 start"
    echo ""
    echo "  # Connect XMRig to the pool"
    echo "  xmrig -o 127.0.0.1:3333 -u <WALLET> -p worker1 -a rx/0"
    echo ""
    echo "  # Check pool stats"
    echo "  curl http://localhost:8080/api/stats"
    echo ""
}

# Main
print_banner

case "${1:-help}" in
    start)
        check_docker
        check_binary
        start_infrastructure
        echo ""
        run_pool
        ;;
    infra|infrastructure)
        check_docker
        start_infrastructure
        ;;
    pool|server)
        check_binary
        run_pool
        ;;
    stop)
        check_docker
        stop_infrastructure
        ;;
    status)
        show_status
        ;;
    help|--help|-h|*)
        show_help
        ;;
esac

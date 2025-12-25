# OpenSY Mining Infrastructure Makefile
# =====================================

.PHONY: all build test clean docker-up docker-down randomx deps lint fmt help

# Variables
GO := go
GOFLAGS := -v
BUILD_DIR := ./build
RANDOMX_DIR := ./common/randomx
DOCKER_COMPOSE := docker compose -f docker/docker-compose.yml

# Default target
all: deps randomx build test

# Build RandomX C library
randomx:
	@echo "==> Building RandomX library..."
	$(MAKE) -C $(RANDOMX_DIR)

# Download dependencies
deps:
	@echo "==> Downloading Go dependencies..."
	$(GO) mod download
	$(GO) mod verify

# Build all binaries
build: deps
	@echo "==> Building binaries..."
	@mkdir -p bin
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o bin/server ./pool/cmd/server
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o bin/coordinator ./coopmine/cmd/coordinator
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o bin/worker ./coopmine/cmd/worker
	@echo "==> Binaries built in bin/"

# Build with version info
build-release: deps
	@echo "==> Building release binaries..."
	@mkdir -p bin
	CGO_ENABLED=1 $(GO) build -ldflags "-X main.Version=$(VERSION) -X main.Commit=$$(git rev-parse --short HEAD) -X main.BuildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/server ./pool/cmd/server
	CGO_ENABLED=1 $(GO) build -ldflags "-X main.Version=$(VERSION) -X main.Commit=$$(git rev-parse --short HEAD) -X main.BuildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/coordinator ./coopmine/cmd/coordinator
	CGO_ENABLED=1 $(GO) build -ldflags "-X main.Version=$(VERSION) -X main.Commit=$$(git rev-parse --short HEAD) -X main.BuildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/worker ./coopmine/cmd/worker
	@echo "==> Release binaries built in bin/"

VERSION ?= 1.0.0

# Build for specific platform
build-linux-amd64:
	@echo "==> Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-amd64/ ./...

build-linux-arm64:
	@echo "==> Building for Linux ARM64..."
	@mkdir -p $(BUILD_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-arm64/ ./...

# Run tests
test:
	@echo "==> Running tests..."
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -race ./...

# Run tests with coverage
test-coverage:
	@echo "==> Running tests with coverage..."
	CGO_ENABLED=1 $(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run RandomX-specific tests
test-randomx:
	@echo "==> Running RandomX tests..."
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -v ./common/randomx/...

# Benchmark RandomX
bench-randomx:
	@echo "==> Benchmarking RandomX..."
	CGO_ENABLED=1 $(GO) test -bench=. -benchmem ./common/randomx/...

# Format code
fmt:
	@echo "==> Formatting code..."
	$(GO) fmt ./...
	gofumpt -l -w .

# Lint code
lint:
	@echo "==> Linting code..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "==> Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(MAKE) -C $(RANDOMX_DIR) clean

# Docker commands
docker-up:
	@echo "==> Starting Docker environment..."
	$(DOCKER_COMPOSE) up -d

docker-down:
	@echo "==> Stopping Docker environment..."
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-ps:
	$(DOCKER_COMPOSE) ps

docker-clean:
	@echo "==> Cleaning Docker environment..."
	$(DOCKER_COMPOSE) down -v --remove-orphans

# Development helpers
dev: docker-up
	@echo "==> Development environment ready!"
	@echo "    PostgreSQL: localhost:5432"
	@echo "    Redis: localhost:6379"
	@echo "    Prometheus: http://localhost:9090"
	@echo "    Grafana: http://localhost:3001"

# Database operations
db-migrate:
	@echo "==> Running database migrations..."
	$(GO) run ./cmd/migrate up

db-rollback:
	@echo "==> Rolling back last migration..."
	$(GO) run ./cmd/migrate down

# Generate Stratum protocol code (if using protobuf)
proto:
	@echo "==> Generating protocol buffers..."
	protoc --go_out=. --go-grpc_out=. proto/*.proto

# Help
help:
	@echo "OpenSY Mining Infrastructure"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  all            Build everything (default)"
	@echo "  build          Build all binaries to bin/"
	@echo "  build-release  Build with version info"
	@echo "  randomx        Build RandomX C library"
	@echo "  deps           Download Go dependencies"
	@echo "  clean          Clean build artifacts"
	@echo ""
	@echo "Test:"
	@echo "  test           Run all tests"
	@echo "  test-randomx   Run RandomX-specific tests"
	@echo "  bench-randomx  Benchmark RandomX"
	@echo "  test-coverage  Run tests with coverage"
	@echo ""
	@echo "Docker:"
	@echo "  docker-up      Start Docker environment"
	@echo "  docker-down    Stop Docker environment"
	@echo "  docker-logs    View Docker logs"
	@echo "  docker-clean   Remove Docker volumes"
	@echo "  dev            Start dev environment with info"
	@echo ""
	@echo "Run (after build):"
	@echo "  ./scripts/run-pool.sh start       - Start mining pool"
	@echo "  ./scripts/run-coopmine.sh coord   - Start CoopMine coordinator"
	@echo "  ./scripts/run-coopmine.sh worker  - Start CoopMine worker"
	@echo ""
	@echo "Code:"
	@echo "  fmt            Format code"
	@echo "  lint           Lint code"

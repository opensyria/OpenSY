# CoopMine - Cooperative Mining System

CoopMine is a distributed cooperative mining system for OpenSY blockchain that enables multiple machines to join a coordinated mining cluster, share work efficiently, and submit results as one unified miner.

## 🎯 Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Mining Pool                              │
│                    (Stratum v1 Protocol)                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       COORDINATOR                                │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │ Pool Client │  │ Job Manager  │  │ Share Aggregator       │  │
│  │ (Stratum)   │  │              │  │ (Validates & Submits)  │  │
│  └─────────────┘  └──────────────┘  └────────────────────────┘  │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │ gRPC Server │  │ Web Dashboard│  │ Worker Registry        │  │
│  │ (TLS)       │  │ (WebSocket)  │  │                        │  │
│  └─────────────┘  └──────────────┘  └────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
          │ gRPC (TLS)          │ gRPC (TLS)          │ gRPC (TLS)
          ▼                     ▼                     ▼
    ┌──────────┐          ┌──────────┐          ┌──────────┐
    │ WORKER 1 │          │ WORKER 2 │          │ WORKER N │
    │ RandomX  │          │ RandomX  │          │ RandomX  │
    │ 4 threads│          │ 8 threads│          │ N threads│
    └──────────┘          └──────────┘          └──────────┘
```

## 🚀 Features

- **Distributed Mining**: Coordinate multiple machines for combined hashrate
- **RandomX Support**: Native RandomX mining via CGO bindings
- **Stratum v1**: Compatible with standard mining pools
- **gRPC Communication**: Efficient binary protocol with TLS encryption
- **Web Dashboard**: Real-time monitoring with WebSocket updates
- **Auto-Recovery**: Automatic reconnection and failover
- **PPLNS Ready**: Share tracking for pay-per-last-N-shares

## 📦 Installation

### Prerequisites

- Go 1.24+
- RandomX library (for worker nodes)
- OpenSSL (for TLS certificates)

### Build from Source

```bash
# Clone the repository
cd /path/to/opensy-mining

# Build coordinator (no CGO required)
go build -o bin/coopmine-coordinator ./coopmine/cmd/coordinator

# Build worker (requires RandomX)
CGO_ENABLED=1 go build -tags "cgo randomx" -o bin/coopmine-worker ./coopmine/cmd/worker
```

### Generate TLS Certificates

```bash
# Generate CA and certificates
./scripts/gen_certs.sh

# Certificates will be created in ./certs/
# - ca.crt          (Certificate Authority)
# - server.crt/key  (Coordinator)
# - client.crt/key  (Workers)
```

## ⚙️ Configuration

### Coordinator Configuration

Create `config/coordinator.yaml`:

```yaml
cluster:
  id: "my-cluster"
  name: "OpenSY Mining Cluster"

pool:
  address: "stratum+tcp://pool.example.com:3333"
  wallet: "YOUR_WALLET_ADDRESS"
  password: "x"

grpc:
  listen: ":50051"
  tls:
    enabled: true
    cert: "./certs/server.crt"
    key: "./certs/server.key"
    ca: "./certs/ca.crt"

dashboard:
  listen: ":8080"
  static_dir: "./coopmine/dashboard/static"

heartbeat_interval: 30s
job_timeout: 5m
```

### Worker Configuration

Create `config/worker.yaml`:

```yaml
worker:
  id: "worker-1"
  name: "AWS c5.xlarge Node"

coordinator:
  address: "coordinator.example.com:50051"
  tls:
    enabled: true
    cert: "./certs/client.crt"
    key: "./certs/client.key"
    ca: "./certs/ca.crt"

mining:
  threads: 4
  huge_pages: true
```

## 🖥️ Usage

### Start Coordinator

```bash
# Using config file
./bin/coopmine-coordinator --config config/coordinator.yaml

# Using environment variables
export COOPMINE_CLUSTER_ID="my-cluster"
export COOPMINE_POOL_ADDRESS="stratum+tcp://pool.example.com:3333"
export COOPMINE_POOL_WALLET="YOUR_WALLET"
./bin/coopmine-coordinator

# Using flags
./bin/coopmine-coordinator \
  --cluster-id "my-cluster" \
  --pool-address "stratum+tcp://pool.example.com:3333" \
  --pool-wallet "YOUR_WALLET" \
  --grpc-listen ":50051" \
  --dashboard-listen ":8080"
```

### Start Worker

```bash
# Using config file
./bin/coopmine-worker --config config/worker.yaml

# Using flags
./bin/coopmine-worker \
  --worker-id "worker-1" \
  --worker-name "My Mining Rig" \
  --coordinator "coordinator.example.com:50051" \
  --threads 4
```

### Access Dashboard

Open your browser and navigate to:

```
http://localhost:8080
```

The dashboard provides:
- Real-time cluster statistics
- Worker status and hashrates
- Share submission history
- Job distribution monitoring

## 🔌 API Reference

### gRPC API

The coordinator exposes the following gRPC services:

```protobuf
service CoopMine {
  // Worker registration
  rpc Register(RegisterRequest) returns (RegisterResponse);
  
  // Heartbeat with hashrate update
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  
  // Submit mining share
  rpc SubmitShare(ShareRequest) returns (ShareResponse);
  
  // Stream mining jobs
  rpc StreamJobs(StreamJobsRequest) returns (stream JobMessage);
  
  // Get cluster statistics
  rpc GetClusterStats(ClusterStatsRequest) returns (ClusterStatsResponse);
  
  // Get worker statistics
  rpc GetWorkerStats(WorkerStatsRequest) returns (WorkerStatsResponse);
}
```

### REST API

The dashboard server provides:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/stats` | GET | Cluster statistics |
| `/api/workers` | GET | List all workers |
| `/api/workers/{id}` | GET | Worker details |
| `/api/jobs` | GET | Recent jobs |
| `/api/shares` | GET | Recent shares |
| `/ws` | WebSocket | Real-time updates |

## 📊 Monitoring

### Prometheus Metrics

```
# HELP coopmine_workers_total Total number of registered workers
# TYPE coopmine_workers_total gauge
coopmine_workers_total 5

# HELP coopmine_workers_online Number of online workers
# TYPE coopmine_workers_online gauge
coopmine_workers_online 4

# HELP coopmine_hashrate_total Total cluster hashrate in H/s
# TYPE coopmine_hashrate_total gauge
coopmine_hashrate_total 15234.56

# HELP coopmine_shares_total Total shares submitted
# TYPE coopmine_shares_total counter
coopmine_shares_total{status="valid"} 1234
coopmine_shares_total{status="invalid"} 12

# HELP coopmine_blocks_found Total blocks found
# TYPE coopmine_blocks_found counter
coopmine_blocks_found 2
```

### Health Check

```bash
# Coordinator health
curl http://localhost:8080/health

# Response
{"status": "healthy", "workers": 5, "pool_connected": true}
```

## 🐳 Docker Deployment

### Build Images

```bash
# Build coordinator
docker build -t coopmine-coordinator -f docker/Dockerfile.coordinator .

# Build worker
docker build -t coopmine-worker -f docker/Dockerfile.worker .
```

### Docker Compose

```yaml
version: '3.8'

services:
  coordinator:
    image: coopmine-coordinator
    ports:
      - "50051:50051"
      - "8080:8080"
    volumes:
      - ./config:/app/config
      - ./certs:/app/certs
    environment:
      - COOPMINE_POOL_WALLET=${WALLET_ADDRESS}

  worker:
    image: coopmine-worker
    deploy:
      replicas: 3
    volumes:
      - ./certs:/app/certs
    environment:
      - COOPMINE_COORDINATOR=coordinator:50051
      - COOPMINE_THREADS=4
```

## 🔧 Development

### Run Tests

```bash
# Unit tests
go test ./coopmine/... -v

# With coverage
go test ./coopmine/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out

# Integration tests (requires RandomX)
CGO_ENABLED=1 go test -tags "cgo randomx" ./coopmine/... -v
```

### Project Structure

```
coopmine/
├── coordinator.go      # Coordinator logic
├── worker.go           # Worker with RandomX mining
├── worker_stub.go      # Stub for non-CGO builds
├── pool_client.go      # Stratum v1 pool client
├── grpc_server.go      # gRPC server implementation
├── grpc_client.go      # gRPC client for workers
├── service.go          # Unified service wrapper
├── coordinator_test.go # Integration tests
├── cmd/
│   ├── coordinator/    # Coordinator binary
│   └── worker/         # Worker binary
├── dashboard/
│   ├── server.go       # Dashboard HTTP server
│   └── static/         # Frontend assets
└── proto/
    ├── coopmine.proto  # Protocol definition
    └── gen/            # Generated Go code
```

## 🔒 Security

### TLS Configuration

All gRPC communication is encrypted with mutual TLS:

1. **Server Authentication**: Workers verify coordinator identity
2. **Client Authentication**: Coordinator verifies worker identity
3. **Encrypted Transport**: All data encrypted in transit

### Best Practices

- Keep private keys secure and never commit to git
- Rotate certificates periodically
- Use strong passwords for pool authentication
- Run coordinator behind firewall with only necessary ports exposed

## 📄 License

This project is part of OpenSY and is licensed under the same terms.

## 🤝 Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.

## 📞 Support

- GitHub Issues: Report bugs and feature requests
- Discord: Join our community for discussions

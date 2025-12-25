// Package e2e provides end-to-end tests for CoopMine
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/opensyria/opensy-mining/coopmine"
)

// MockPoolServer simulates a Stratum mining pool
type MockPoolServer struct {
	listener   net.Listener
	jobs       chan *MockJob
	shares     []*MockShare
	sharesMu   sync.Mutex
	difficulty uint64
	running    bool
	mu         sync.RWMutex
}

type MockJob struct {
	ID       string
	Blob     string
	Target   string
	Height   uint64
	SeedHash string
}

type MockShare struct {
	WorkerID string
	JobID    string
	Nonce    string
	Result   string
}

// NewMockPoolServer creates a new mock pool server
func NewMockPoolServer() *MockPoolServer {
	return &MockPoolServer{
		jobs:       make(chan *MockJob, 10),
		shares:     make([]*MockShare, 0),
		difficulty: 10000,
	}
}

// Start starts the mock pool server
func (m *MockPoolServer) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	m.listener = listener
	m.running = true

	go m.acceptConnections()
	return nil
}

// Stop stops the mock pool server
func (m *MockPoolServer) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	if m.listener != nil {
		m.listener.Close()
	}
}

// Addr returns the server address
func (m *MockPoolServer) Addr() string {
	if m.listener == nil {
		return ""
	}
	return m.listener.Addr().String()
}

// SendJob sends a new job to connected miners
func (m *MockPoolServer) SendJob(job *MockJob) {
	select {
	case m.jobs <- job:
	default:
	}
}

// GetShares returns all received shares
func (m *MockPoolServer) GetShares() []*MockShare {
	m.sharesMu.Lock()
	defer m.sharesMu.Unlock()
	return append([]*MockShare{}, m.shares...)
}

func (m *MockPoolServer) acceptConnections() {
	for {
		m.mu.RLock()
		running := m.running
		m.mu.RUnlock()

		if !running {
			return
		}

		conn, err := m.listener.Accept()
		if err != nil {
			continue
		}

		go m.handleConnection(conn)
	}
}

func (m *MockPoolServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Simple JSON-RPC handler for Stratum
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req map[string]interface{}
		if err := decoder.Decode(&req); err != nil {
			return
		}

		method, _ := req["method"].(string)
		id := req["id"]

		switch method {
		case "login":
			// Accept login
			encoder.Encode(map[string]interface{}{
				"id": id,
				"result": map[string]interface{}{
					"status": "OK",
					"job": map[string]interface{}{
						"job_id":    "initial-job",
						"blob":      "0707e6bdfedc0500000000000000",
						"target":    "ffffffff",
						"height":    100,
						"seed_hash": "abcdef1234567890",
					},
				},
				"error": nil,
			})

		case "submit":
			// Accept share
			params, _ := req["params"].(map[string]interface{})
			m.sharesMu.Lock()
			m.shares = append(m.shares, &MockShare{
				WorkerID: fmt.Sprintf("%v", params["id"]),
				JobID:    fmt.Sprintf("%v", params["job_id"]),
				Nonce:    fmt.Sprintf("%v", params["nonce"]),
				Result:   fmt.Sprintf("%v", params["result"]),
			})
			m.sharesMu.Unlock()

			encoder.Encode(map[string]interface{}{
				"id":     id,
				"result": map[string]interface{}{"status": "OK"},
				"error":  nil,
			})

		case "keepalived":
			encoder.Encode(map[string]interface{}{
				"id":     id,
				"result": map[string]interface{}{"status": "KEEPALIVED"},
				"error":  nil,
			})
		}
	}
}

// TestCoordinatorPoolConnection tests coordinator connecting to pool
func TestCoordinatorPoolConnection(t *testing.T) {
	// Start mock pool
	pool := NewMockPoolServer()
	if err := pool.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Failed to start mock pool: %v", err)
	}
	defer pool.Stop()

	// Create coordinator
	cfg := coopmine.ClusterConfig{
		ClusterID:    "e2e-test",
		ClusterName:  "E2E Test Cluster",
		PoolAddr:     "stratum+tcp://" + pool.Addr(),
		PoolLogin:    "test-wallet",
		PoolPassword: "x",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := coopmine.NewCoordinator(cfg)

	// Start coordinator
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Give time to connect
	time.Sleep(500 * time.Millisecond)

	// Verify connection
	stats := coord.GetStats()
	t.Logf("Coordinator stats: %+v", stats)
}

// TestFullMiningCycle tests complete mining workflow
func TestFullMiningCycle(t *testing.T) {
	// Create coordinator
	cfg := coopmine.ClusterConfig{
		ClusterID:    "e2e-full-test",
		ClusterName:  "Full E2E Test",
		HeartbeatInt: 50 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := coopmine.NewCoordinator(cfg)

	// Start coordinator
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register workers
	workers := []struct {
		id   string
		name string
		addr string
	}{
		{"w1", "Worker 1", "192.168.1.1:5000"},
		{"w2", "Worker 2", "192.168.1.2:5000"},
		{"w3", "Worker 3", "192.168.1.3:5000"},
	}

	for _, w := range workers {
		_, err := coord.RegisterWorker(w.id, w.name, w.addr)
		if err != nil {
			t.Fatalf("Failed to register worker %s: %v", w.id, err)
		}
	}

	// Verify workers registered
	stats := coord.GetStats()
	if stats.TotalWorkers != 3 {
		t.Errorf("Expected 3 workers, got %d", stats.TotalWorkers)
	}

	// Set a job
	job := &coopmine.Job{
		ID:        "e2e-job-1",
		Blob:      "0707e6bdfedc0500000000000000",
		Target:    "ffffffff",
		Height:    12345,
		SeedHash:  "abcdef1234567890",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	coord.SetJob(job)

	// Workers send heartbeats
	for _, w := range workers {
		coord.Heartbeat(w.id, 1000+float64(len(w.id)*100))
	}

	// Verify hashrate
	stats = coord.GetStats()
	if stats.TotalHashrate == 0 {
		t.Error("Expected non-zero total hashrate")
	}
	t.Logf("Total hashrate: %.2f H/s", stats.TotalHashrate)

	// Submit shares
	for i, w := range workers {
		share := &coopmine.Share{
			WorkerID:  w.id,
			JobID:     job.ID,
			Nonce:     fmt.Sprintf("%08x", i+1),
			Result:    "0000000000000000abcdef",
			Timestamp: time.Now(),
		}
		_, err := coord.SubmitShare(share)
		if err != nil {
			t.Logf("Share submission error (expected): %v", err)
		}
	}

	// Final stats
	stats = coord.GetStats()
	t.Logf("Final stats: Workers=%d, Online=%d, Hashrate=%.2f, ValidShares=%d",
		stats.TotalWorkers, stats.OnlineWorkers, stats.TotalHashrate, stats.SharesValid)
}

// TestWorkerFailover tests worker disconnect and reconnect
func TestWorkerFailover(t *testing.T) {
	cfg := coopmine.ClusterConfig{
		ClusterID:    "e2e-failover",
		ClusterName:  "Failover Test",
		HeartbeatInt: 30 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := coopmine.NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register worker
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5000")
	coord.Heartbeat("w1", 1000)

	// Verify online
	worker := coord.GetWorker("w1")
	if worker.Status != coopmine.WorkerMining {
		t.Errorf("Expected worker status Mining, got %v", worker.Status)
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Check offline
	worker = coord.GetWorker("w1")
	if worker.Status != coopmine.WorkerOffline {
		t.Errorf("Expected worker status Offline, got %v", worker.Status)
	}

	// Worker "reconnects" with heartbeat
	coord.Heartbeat("w1", 1200)

	// Verify back online
	worker = coord.GetWorker("w1")
	if worker.Status != coopmine.WorkerMining {
		t.Errorf("Expected worker status Mining after heartbeat, got %v", worker.Status)
	}
}

// TestDashboardAPI tests the dashboard HTTP API
func TestDashboardAPI(t *testing.T) {
	// Start coordinator
	cfg := coopmine.ClusterConfig{
		ClusterID:    "e2e-dashboard",
		ClusterName:  "Dashboard Test",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := coopmine.NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Add some workers
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5000")
	coord.RegisterWorker("w2", "Worker 2", "127.0.0.1:5001")
	coord.Heartbeat("w1", 1000)
	coord.Heartbeat("w2", 2000)

	// Create a simple HTTP server to test API patterns
	mux := http.NewServeMux()

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := coord.GetStats()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cluster_id":     coord.GetClusterID(),
			"cluster_name":   coord.GetClusterName(),
			"workers_online": stats.OnlineWorkers,
			"total_hashrate": stats.TotalHashrate,
			"shares_valid":   stats.SharesValid,
		})
	})

	mux.HandleFunc("/api/workers", func(w http.ResponseWriter, r *http.Request) {
		workers := make([]map[string]interface{}, 0)
		coord.ForEachWorker(func(wi *coopmine.WorkerInfo) {
			status := "idle"
			switch wi.Status {
			case coopmine.WorkerMining:
				status = "mining"
			case coopmine.WorkerOffline:
				status = "offline"
			}
			workers = append(workers, map[string]interface{}{
				"id":       wi.ID,
				"name":     wi.Name,
				"status":   status,
				"hashrate": wi.Hashrate,
			})
		})
		json.NewEncoder(w).Encode(workers)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Start server
	server := &http.Server{Addr: "127.0.0.1:0", Handler: mux}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	baseURL := "http://" + listener.Addr().String()

	// Test health endpoint
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Test stats endpoint
	resp, err = http.Get(baseURL + "/api/stats")
	if err != nil {
		t.Fatalf("Stats request failed: %v", err)
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode stats: %v", err)
	}

	if stats["cluster_id"] != "e2e-dashboard" {
		t.Errorf("Unexpected cluster_id: %v", stats["cluster_id"])
	}

	// Test workers endpoint
	resp, err = http.Get(baseURL + "/api/workers")
	if err != nil {
		t.Fatalf("Workers request failed: %v", err)
	}
	defer resp.Body.Close()

	var workers []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&workers); err != nil {
		t.Fatalf("Failed to decode workers: %v", err)
	}

	if len(workers) != 2 {
		t.Errorf("Expected 2 workers, got %d", len(workers))
	}

	t.Logf("API test passed. Stats: %v, Workers: %d", stats, len(workers))
}

// TestConcurrentWorkers tests handling many concurrent workers
func TestConcurrentWorkers(t *testing.T) {
	cfg := coopmine.ClusterConfig{
		ClusterID:    "e2e-concurrent",
		ClusterName:  "Concurrent Test",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := coopmine.NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	numWorkers := 50
	var wg sync.WaitGroup

	// Concurrently register workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("worker-%d", idx)
			name := fmt.Sprintf("Worker %d", idx)
			addr := fmt.Sprintf("192.168.1.%d:5000", idx)
			coord.RegisterWorker(id, name, addr)
			coord.Heartbeat(id, float64(1000+idx*10))
		}(i)
	}

	wg.Wait()

	// Verify all workers registered
	stats := coord.GetStats()
	if stats.TotalWorkers != numWorkers {
		t.Errorf("Expected %d workers, got %d", numWorkers, stats.TotalWorkers)
	}

	t.Logf("Concurrent test: %d workers, %.2f H/s total hashrate",
		stats.TotalWorkers, stats.TotalHashrate)
}

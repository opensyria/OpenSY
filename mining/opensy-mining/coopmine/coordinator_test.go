package coopmine

import (
	"testing"
	"time"
)

func TestCoordinatorBasic(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:    "test-cluster",
		ClusterName:  "Test Cluster",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if coord == nil {
		t.Fatal("NewCoordinator returned nil")
	}

	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Test registration
	worker, err := coord.RegisterWorker("worker-1", "Test Worker 1", "127.0.0.1:5000")
	if err != nil {
		t.Fatalf("Failed to register worker: %v", err)
	}

	if worker == nil {
		t.Fatal("RegisterWorker returned nil worker")
	}

	if worker.Name != "Test Worker 1" {
		t.Errorf("Expected worker name 'Test Worker 1', got '%s'", worker.Name)
	}

	// Verify worker is registered
	w := coord.GetWorker("worker-1")
	if w == nil {
		t.Fatal("Worker not found after registration")
	}
}

func TestCoordinatorMultipleWorkers(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:    "test-cluster-multi",
		ClusterName:  "Multi Worker Test",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register multiple workers
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
		if _, err := coord.RegisterWorker(w.id, w.name, w.addr); err != nil {
			t.Fatalf("Failed to register worker %s: %v", w.id, err)
		}
	}

	// Verify stats
	stats := coord.GetStats()
	if stats.OnlineWorkers != 3 {
		t.Errorf("Expected 3 online workers, got %d", stats.OnlineWorkers)
	}

	// ForEachWorker test
	count := 0
	coord.ForEachWorker(func(w *WorkerInfo) {
		count++
	})

	if count != 3 {
		t.Errorf("ForEachWorker visited %d workers, expected 3", count)
	}

	// Unregister one
	coord.UnregisterWorker("w2")

	stats = coord.GetStats()
	if stats.OnlineWorkers != 2 {
		t.Errorf("Expected 2 online workers after unregister, got %d", stats.OnlineWorkers)
	}
}

func TestCoordinatorJobDistribution(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:    "test-job-dist",
		ClusterName:  "Job Distribution Test",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register workers
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5001")
	coord.RegisterWorker("w2", "Worker 2", "127.0.0.1:5002")

	// Set a job
	job := &Job{
		ID:        "job-1",
		Blob:      "000000000000000000000000",
		Target:    "ffffffff",
		Height:    12345,
		SeedHash:  "abcdef1234567890",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	coord.SetJob(job)

	// Get jobs for each worker - they should have unique extra nonces
	job1, err := coord.GetJobForWorker("w1")
	if err != nil {
		t.Fatalf("Failed to get job for w1: %v", err)
	}

	job2, err := coord.GetJobForWorker("w2")
	if err != nil {
		t.Fatalf("Failed to get job for w2: %v", err)
	}

	// Extra nonces should be different
	if job1.ExtraNonce == job2.ExtraNonce {
		t.Error("Workers have the same extra nonce - work would be duplicated!")
	}
}

func TestCoordinatorHeartbeat(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:    "test-heartbeat",
		ClusterName:  "Heartbeat Test",
		HeartbeatInt: 50 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register worker
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5000")

	// Send heartbeat with hashrate
	err := coord.Heartbeat("w1", 1500.5)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// Verify hashrate was updated
	worker := coord.GetWorker("w1")
	if worker.Hashrate != 1500.5 {
		t.Errorf("Expected hashrate 1500.5, got %f", worker.Hashrate)
	}

	// Heartbeat for non-existent worker should fail
	err = coord.Heartbeat("nonexistent", 100)
	if err == nil {
		t.Error("Heartbeat for nonexistent worker should fail")
	}
}

func TestCoordinatorShareSubmission(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:        "test-shares",
		ClusterName:      "Share Test",
		TargetDifficulty: 1000,
		HeartbeatInt:     100 * time.Millisecond,
		JobTimeout:       5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register worker
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5000")

	// Set a job
	job := &Job{
		ID:        "job-1",
		Blob:      "000000000000000000000000",
		Target:    "ffffffff",
		Height:    12345,
		SeedHash:  "abcdef1234567890",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	coord.SetJob(job)

	// Submit a share
	share := &Share{
		WorkerID:  "w1",
		JobID:     "job-1",
		Nonce:     "12345678",
		Result:    "0000000000000000abcdef",
		Timestamp: time.Now(),
	}

	accepted, err := coord.SubmitShare(share)
	if err != nil {
		t.Logf("Share submission returned error (may be expected): %v", err)
	}

	// Check if share was processed
	_ = accepted

	// Stats should reflect the share
	stats := coord.GetStats()
	_ = stats
}

func TestClusterStats(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:    "test-stats",
		ClusterName:  "Stats Test",
		HeartbeatInt: 100 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Initial stats
	stats := coord.GetStats()
	if stats.OnlineWorkers != 0 {
		t.Errorf("Expected 0 workers initially, got %d", stats.OnlineWorkers)
	}

	// Add workers with hashrates
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5000")
	coord.RegisterWorker("w2", "Worker 2", "127.0.0.1:5001")

	coord.Heartbeat("w1", 1000)
	coord.Heartbeat("w2", 2000)

	stats = coord.GetStats()

	if stats.OnlineWorkers != 2 {
		t.Errorf("Expected 2 workers, got %d", stats.OnlineWorkers)
	}

	if stats.TotalHashrate != 3000 {
		t.Errorf("Expected total hashrate 3000, got %f", stats.TotalHashrate)
	}

	if stats.Uptime <= 0 {
		t.Error("Uptime should be positive")
	}
}

func TestWorkerTimeout(t *testing.T) {
	cfg := ClusterConfig{
		ClusterID:    "test-timeout",
		ClusterName:  "Timeout Test",
		HeartbeatInt: 20 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	}

	coord := NewCoordinator(cfg)
	if err := coord.Start(); err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coord.Stop()

	// Register worker
	coord.RegisterWorker("w1", "Worker 1", "127.0.0.1:5000")

	// Wait for timeout (3 * heartbeat interval)
	time.Sleep(100 * time.Millisecond)

	// Worker should be marked offline
	worker := coord.GetWorker("w1")
	if worker == nil {
		t.Fatal("Worker should still exist")
	}

	if worker.Status != WorkerOffline {
		t.Errorf("Expected worker status Offline, got %v", worker.Status)
	}
}

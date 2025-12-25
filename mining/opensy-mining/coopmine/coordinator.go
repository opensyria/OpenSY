// Package coopmine implements the CoopMine cooperative mining system.
// CoopMine enables multiple machines to join a coordinated mining group,
// share work efficiently, and submit results as one unified miner.
package coopmine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ClusterConfig holds cluster configuration
type ClusterConfig struct {
	ClusterID        string
	ClusterName      string
	CoordinatorAddr  string
	PoolAddr         string // Upstream pool address
	PoolLogin        string // Pool wallet address
	PoolPassword     string
	TargetDifficulty uint64
	HeartbeatInt     time.Duration
	WorkerTimeout    time.Duration
	JobTimeout       time.Duration
	Logger           *slog.Logger
}

// DefaultClusterConfig returns default configuration
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		HeartbeatInt: 10 * time.Second,
		JobTimeout:   30 * time.Second,
		Logger:       slog.Default(),
	}
}

// WorkerInfo represents a connected worker node
type WorkerInfo struct {
	ID            string
	Name          string
	Addr          string
	Hashrate      float64
	SharesValid   uint64
	SharesInvalid uint64
	LastSeen      time.Time
	JoinedAt      time.Time
	CurrentJob    string
	Status        WorkerStatus
}

// WorkerStatus represents worker state
type WorkerStatus int

const (
	WorkerIdle WorkerStatus = iota
	WorkerMining
	WorkerOffline
)

// Job represents a mining job distributed to workers
type Job struct {
	ID         string
	Blob       string // Block header template
	Target     string // Mining target (compact)
	Height     int64
	SeedHash   string
	Algo       string
	ExtraNonce uint32 // Unique per worker to avoid duplicate work
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// Share represents a share submitted by a worker
type Share struct {
	WorkerID  string
	JobID     string
	Nonce     string
	Result    string
	Timestamp time.Time
}

// ClusterStats holds cluster-wide statistics
type ClusterStats struct {
	ClusterID     string
	TotalWorkers  int
	OnlineWorkers int
	TotalHashrate float64
	SharesValid   uint64
	SharesInvalid uint64
	BlocksFound   uint64
	Uptime        time.Duration
	LastBlockTime *time.Time
}

// Coordinator manages the CoopMine cluster
type Coordinator struct {
	cfg    ClusterConfig
	logger *slog.Logger

	// Workers
	workers   map[string]*WorkerInfo
	workersMu sync.RWMutex

	// Jobs
	currentJob *Job
	jobHistory map[string]*Job
	jobMu      sync.RWMutex
	extraNonce atomic.Uint32

	// Stats
	sharesValid   atomic.Uint64
	sharesInvalid atomic.Uint64
	blocksFound   atomic.Uint64
	startTime     time.Time

	// Upstream pool connection (handled separately)
	poolJobChan chan *Job

	// Callbacks
	OnShareAccepted func(jobID, nonce, result string) (bool, error)

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCoordinator creates a new cluster coordinator
func NewCoordinator(cfg ClusterConfig) *Coordinator {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.ClusterID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		cfg.ClusterID = hex.EncodeToString(b)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Coordinator{
		cfg:         cfg,
		logger:      cfg.Logger.With("component", "coordinator", "cluster", cfg.ClusterID),
		workers:     make(map[string]*WorkerInfo),
		jobHistory:  make(map[string]*Job),
		poolJobChan: make(chan *Job, 10),
		startTime:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the coordinator
func (c *Coordinator) Start() error {
	c.logger.Info("Starting CoopMine coordinator",
		"cluster", c.cfg.ClusterID,
		"pool", c.cfg.PoolAddr,
	)

	// Start worker health check loop
	c.wg.Add(1)
	go c.healthCheckLoop()

	// Start stats reporting loop
	c.wg.Add(1)
	go c.statsLoop()

	c.logger.Info("Coordinator started")
	return nil
}

// Stop stops the coordinator
func (c *Coordinator) Stop() {
	c.logger.Info("Stopping coordinator")
	c.cancel()
	c.wg.Wait()
	c.logger.Info("Coordinator stopped")
}

// RegisterWorker registers a new worker in the cluster
func (c *Coordinator) RegisterWorker(id, name, addr string) (*WorkerInfo, error) {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	if _, exists := c.workers[id]; exists {
		return nil, fmt.Errorf("worker %s already registered", id)
	}

	worker := &WorkerInfo{
		ID:       id,
		Name:     name,
		Addr:     addr,
		LastSeen: time.Now(),
		JoinedAt: time.Now(),
		Status:   WorkerIdle,
	}

	c.workers[id] = worker

	c.logger.Info("Worker registered",
		"worker_id", id,
		"name", name,
		"addr", addr,
		"total_workers", len(c.workers),
	)

	return worker, nil
}

// UnregisterWorker removes a worker from the cluster
func (c *Coordinator) UnregisterWorker(id string) {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	if worker, exists := c.workers[id]; exists {
		c.logger.Info("Worker unregistered",
			"worker_id", id,
			"name", worker.Name,
			"shares", worker.SharesValid,
		)
		delete(c.workers, id)
	}
}

// Heartbeat updates worker's last seen time and stats
func (c *Coordinator) Heartbeat(id string, hashrate float64) error {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	worker, exists := c.workers[id]
	if !exists {
		return fmt.Errorf("worker %s not found", id)
	}

	worker.LastSeen = time.Now()
	worker.Hashrate = hashrate
	worker.Status = WorkerMining

	return nil
}

// GetJobForWorker returns the current job with unique extra nonce
func (c *Coordinator) GetJobForWorker(workerID string) (*Job, error) {
	c.jobMu.RLock()
	currentJob := c.currentJob
	c.jobMu.RUnlock()

	if currentJob == nil {
		return nil, fmt.Errorf("no job available")
	}

	c.workersMu.Lock()
	worker, exists := c.workers[workerID]
	if !exists {
		c.workersMu.Unlock()
		return nil, fmt.Errorf("worker %s not found", workerID)
	}
	worker.CurrentJob = currentJob.ID
	worker.Status = WorkerMining
	c.workersMu.Unlock()

	// Create worker-specific job with unique extra nonce
	extraNonce := c.extraNonce.Add(1)
	workerJob := &Job{
		ID:         currentJob.ID,
		Blob:       currentJob.Blob,
		Target:     currentJob.Target,
		Height:     currentJob.Height,
		SeedHash:   currentJob.SeedHash,
		Algo:       currentJob.Algo,
		ExtraNonce: extraNonce,
		CreatedAt:  currentJob.CreatedAt,
		ExpiresAt:  currentJob.ExpiresAt,
	}

	return workerJob, nil
}

// SubmitShare processes a share from a worker
func (c *Coordinator) SubmitShare(share *Share) (bool, error) {
	c.workersMu.Lock()
	worker, exists := c.workers[share.WorkerID]
	if !exists {
		c.workersMu.Unlock()
		return false, fmt.Errorf("worker %s not found", share.WorkerID)
	}
	c.workersMu.Unlock()

	// Verify job exists
	c.jobMu.RLock()
	job, exists := c.jobHistory[share.JobID]
	c.jobMu.RUnlock()

	if !exists {
		c.sharesInvalid.Add(1)
		c.workersMu.Lock()
		worker.SharesInvalid++
		c.workersMu.Unlock()
		return false, fmt.Errorf("job %s not found", share.JobID)
	}

	// Check job expiration
	if time.Now().After(job.ExpiresAt) {
		c.sharesInvalid.Add(1)
		return false, fmt.Errorf("job %s expired", share.JobID)
	}

	// Forward share to upstream pool
	if c.OnShareAccepted != nil {
		accepted, err := c.OnShareAccepted(share.JobID, share.Nonce, share.Result)
		if err != nil {
			c.logger.Warn("Pool rejected share", "err", err)
			c.sharesInvalid.Add(1)
			c.workersMu.Lock()
			worker.SharesInvalid++
			c.workersMu.Unlock()
			return false, err
		}
		if !accepted {
			c.sharesInvalid.Add(1)
			return false, fmt.Errorf("pool rejected share")
		}
	}

	c.sharesValid.Add(1)
	c.workersMu.Lock()
	worker.SharesValid++
	c.workersMu.Unlock()

	c.logger.Debug("Share accepted",
		"worker", share.WorkerID,
		"job", share.JobID,
		"nonce", share.Nonce,
	)

	return true, nil
}

// SetJob sets a new job from the upstream pool
func (c *Coordinator) SetJob(job *Job) {
	c.jobMu.Lock()
	c.currentJob = job
	c.jobHistory[job.ID] = job

	// Clean old jobs
	for id, j := range c.jobHistory {
		if time.Now().After(j.ExpiresAt.Add(5 * time.Minute)) {
			delete(c.jobHistory, id)
		}
	}
	c.jobMu.Unlock()

	c.logger.Info("New job received",
		"job_id", job.ID,
		"height", job.Height,
	)

	// Notify job channel for broadcasting to workers
	select {
	case c.poolJobChan <- job:
	default:
	}
}

// GetStats returns cluster statistics
func (c *Coordinator) GetStats() *ClusterStats {
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()

	var totalHashrate float64
	var onlineWorkers int

	for _, worker := range c.workers {
		if worker.Status != WorkerOffline {
			onlineWorkers++
			totalHashrate += worker.Hashrate
		}
	}

	return &ClusterStats{
		ClusterID:     c.cfg.ClusterID,
		TotalWorkers:  len(c.workers),
		OnlineWorkers: onlineWorkers,
		TotalHashrate: totalHashrate,
		SharesValid:   c.sharesValid.Load(),
		SharesInvalid: c.sharesInvalid.Load(),
		BlocksFound:   c.blocksFound.Load(),
		Uptime:        time.Since(c.startTime),
	}
}

// GetWorkers returns all workers
func (c *Coordinator) GetWorkers() []*WorkerInfo {
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()

	workers := make([]*WorkerInfo, 0, len(c.workers))
	for _, w := range c.workers {
		workers = append(workers, w)
	}
	return workers
}

// JobChannel returns channel for job notifications
func (c *Coordinator) JobChannel() <-chan *Job {
	return c.poolJobChan
}

func (c *Coordinator) healthCheckLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.cfg.HeartbeatInt)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkWorkerHealth()
		}
	}
}

func (c *Coordinator) checkWorkerHealth() {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	timeout := 3 * c.cfg.HeartbeatInt

	for _, worker := range c.workers {
		if time.Since(worker.LastSeen) > timeout {
			if worker.Status != WorkerOffline {
				worker.Status = WorkerOffline
				c.logger.Warn("Worker went offline",
					"worker_id", worker.ID,
					"name", worker.Name,
					"last_seen", worker.LastSeen,
				)
			}
		}
	}
}

func (c *Coordinator) statsLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			stats := c.GetStats()
			c.logger.Info("Cluster stats",
				"workers", stats.OnlineWorkers,
				"hashrate", stats.TotalHashrate,
				"shares", stats.SharesValid,
				"blocks", stats.BlocksFound,
			)
		}
	}
}

// GetClusterID returns the cluster ID
func (c *Coordinator) GetClusterID() string {
	return c.cfg.ClusterID
}

// GetClusterName returns the cluster name
func (c *Coordinator) GetClusterName() string {
	return c.cfg.ClusterName
}

// ForEachWorker iterates over all workers
func (c *Coordinator) ForEachWorker(fn func(*WorkerInfo)) {
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()

	for _, w := range c.workers {
		fn(w)
	}
}

// GetWorker returns a worker by ID
func (c *Coordinator) GetWorker(id string) *WorkerInfo {
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()
	return c.workers[id]
}

// GetWorkerExtraNonce returns the extra nonce for a worker
func (c *Coordinator) GetWorkerExtraNonce(workerID string) uint32 {
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()
	if w, ok := c.workers[workerID]; ok {
		// Use worker index as extra nonce to ensure uniqueness
		var idx uint32
		for id := range c.workers {
			if id == workerID {
				break
			}
			idx++
		}
		_ = w // Use w if needed
		return idx
	}
	return 0
}

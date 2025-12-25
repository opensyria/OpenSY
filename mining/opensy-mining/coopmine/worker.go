//go:build cgo && randomx

// Package coopmine - worker.go implements the worker node that connects to a coordinator
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

	"github.com/opensyria/opensy-mining/common/randomx"
)

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	WorkerID        string
	WorkerName      string
	CoordinatorAddr string
	Threads         int // Mining threads (0 = auto)
	HeartbeatInt    time.Duration
	Logger          *slog.Logger
}

// DefaultWorkerConfig returns default configuration
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		Threads:      0, // Auto-detect
		HeartbeatInt: 10 * time.Second,
		Logger:       slog.Default(),
	}
}

// Worker is a mining worker that connects to a coordinator
type Worker struct {
	cfg    WorkerConfig
	logger *slog.Logger

	// RandomX
	rxCache  *randomx.Cache
	rxVMs    []*randomx.VM
	rxMu     sync.RWMutex
	seedHash string

	// Current job
	currentJob *Job
	jobMu      sync.RWMutex

	// Mining state
	mining    atomic.Bool
	hashCount atomic.Uint64
	startTime time.Time

	// Stats
	sharesValid   atomic.Uint64
	sharesInvalid atomic.Uint64

	// Callbacks
	OnShareFound func(jobID, nonce, result string)
	OnBlockFound func(height int64, hash string)

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWorker creates a new mining worker
func NewWorker(cfg WorkerConfig) *Worker {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.WorkerID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		cfg.WorkerID = hex.EncodeToString(b)
	}

	if cfg.WorkerName == "" {
		cfg.WorkerName = "worker-" + cfg.WorkerID[:4]
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		cfg:       cfg,
		logger:    cfg.Logger.With("component", "worker", "id", cfg.WorkerID),
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the worker
func (w *Worker) Start() error {
	w.logger.Info("Starting CoopMine worker",
		"id", w.cfg.WorkerID,
		"name", w.cfg.WorkerName,
		"coordinator", w.cfg.CoordinatorAddr,
	)

	// Start heartbeat loop
	w.wg.Add(1)
	go w.heartbeatLoop()

	// Start hashrate calculation loop
	w.wg.Add(1)
	go w.hashrateLoop()

	w.logger.Info("Worker started")
	return nil
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.logger.Info("Stopping worker")
	w.mining.Store(false)
	w.cancel()
	w.wg.Wait()

	// Cleanup RandomX
	w.rxMu.Lock()
	for _, vm := range w.rxVMs {
		if vm != nil {
			vm.Close()
		}
	}
	if w.rxCache != nil {
		w.rxCache.Close()
	}
	w.rxMu.Unlock()

	w.logger.Info("Worker stopped")
}

// SetJob sets a new job to mine
func (w *Worker) SetJob(job *Job) error {
	w.jobMu.Lock()
	w.currentJob = job
	w.jobMu.Unlock()

	// Check if seed hash changed
	if job.SeedHash != w.seedHash {
		if err := w.updateSeed(job.SeedHash); err != nil {
			return fmt.Errorf("failed to update seed: %w", err)
		}
	}

	w.logger.Info("New job received",
		"job_id", job.ID,
		"height", job.Height,
		"target", job.Target,
	)

	// Start mining if not already
	if !w.mining.Load() {
		w.startMining()
	}

	return nil
}

func (w *Worker) updateSeed(seedHash string) error {
	seedBytes, err := hex.DecodeString(seedHash)
	if err != nil {
		return fmt.Errorf("invalid seed hash: %w", err)
	}

	w.rxMu.Lock()
	defer w.rxMu.Unlock()

	// Close old VMs
	for _, vm := range w.rxVMs {
		if vm != nil {
			vm.Close()
		}
	}

	// Close old cache
	if w.rxCache != nil {
		w.rxCache.Close()
	}

	// Create new cache
	cache, err := randomx.NewCache(randomx.FlagDefault | randomx.FlagJIT)
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	cache.Init(seedBytes)
	w.rxCache = cache

	// Determine thread count
	threads := w.cfg.Threads
	if threads <= 0 {
		threads = 1 // Default to 1, should detect CPU cores
	}

	// Create VMs (one per thread)
	w.rxVMs = make([]*randomx.VM, threads)
	for i := 0; i < threads; i++ {
		vm, err := randomx.NewVM(cache, nil, randomx.FlagDefault|randomx.FlagJIT)
		if err != nil {
			return fmt.Errorf("failed to create VM %d: %w", i, err)
		}
		w.rxVMs[i] = vm
	}

	w.seedHash = seedHash
	w.logger.Info("RandomX seed updated",
		"seed", seedHash[:16]+"...",
		"threads", threads,
	)

	return nil
}

func (w *Worker) startMining() {
	w.mining.Store(true)

	threads := len(w.rxVMs)
	if threads == 0 {
		threads = 1
	}

	for i := 0; i < threads; i++ {
		w.wg.Add(1)
		go w.miningThread(i)
	}

	w.logger.Info("Mining started", "threads", threads)
}

func (w *Worker) miningThread(threadID int) {
	defer w.wg.Done()

	w.rxMu.RLock()
	if threadID >= len(w.rxVMs) || w.rxVMs[threadID] == nil {
		w.rxMu.RUnlock()
		w.logger.Error("No VM for thread", "thread", threadID)
		return
	}
	vm := w.rxVMs[threadID]
	w.rxMu.RUnlock()

	var nonce uint32 = uint32(threadID) // Start at different points

	for w.mining.Load() {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		w.jobMu.RLock()
		job := w.currentJob
		w.jobMu.RUnlock()

		if job == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Build header with nonce
		header := w.buildHeader(job, nonce)

		// Calculate hash
		hash := vm.CalculateHash(header)
		w.hashCount.Add(1)

		// Check if meets target
		if w.checkTarget(hash, job.Target) {
			nonceHex := fmt.Sprintf("%08x", nonce)
			resultHex := hex.EncodeToString(hash)

			w.logger.Info("Share found!",
				"job", job.ID,
				"nonce", nonceHex,
				"hash", resultHex[:16]+"...",
			)

			w.sharesValid.Add(1)

			if w.OnShareFound != nil {
				w.OnShareFound(job.ID, nonceHex, resultHex)
			}

			// Check if block
			if w.checkBlockTarget(hash, job) {
				w.logger.Info("BLOCK FOUND!",
					"height", job.Height,
					"hash", resultHex,
				)
				if w.OnBlockFound != nil {
					w.OnBlockFound(job.Height, resultHex)
				}
			}
		}

		// Increment nonce, spacing by thread count
		nonce += uint32(len(w.rxVMs))
		if nonce < uint32(threadID) {
			// Wrapped around, job is exhausted for this thread
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (w *Worker) buildHeader(job *Job, nonce uint32) []byte {
	// Decode blob
	blob, err := hex.DecodeString(job.Blob)
	if err != nil {
		return nil
	}

	// Make a copy
	header := make([]byte, len(blob))
	copy(header, blob)

	// Insert nonce at bytes 76-80 (little endian)
	if len(header) >= 80 {
		header[76] = byte(nonce)
		header[77] = byte(nonce >> 8)
		header[78] = byte(nonce >> 16)
		header[79] = byte(nonce >> 24)
	}

	// Insert extra nonce at reserved position if available
	if job.ExtraNonce > 0 && len(header) >= 44 {
		header[40] = byte(job.ExtraNonce)
		header[41] = byte(job.ExtraNonce >> 8)
		header[42] = byte(job.ExtraNonce >> 16)
		header[43] = byte(job.ExtraNonce >> 24)
	}

	return header
}

func (w *Worker) checkTarget(hash []byte, targetHex string) bool {
	// Simplified target check
	// Real implementation would compare full 256-bit values
	if len(hash) < 4 || len(targetHex) < 8 {
		return false
	}

	// Check leading zeros (simplified)
	target, _ := hex.DecodeString(targetHex)
	if len(target) < 4 {
		return false
	}

	// Compare first 4 bytes (little endian)
	for i := 31; i >= 28; i-- {
		if i < len(hash) {
			if hash[i] > target[31-i] {
				return false
			}
			if hash[i] < target[31-i] {
				return true
			}
		}
	}
	return true
}

func (w *Worker) checkBlockTarget(hash []byte, job *Job) bool {
	// Would check against network target, not share target
	// For now, return false - real impl would verify
	return false
}

// GetHashrate returns current hashrate in H/s
func (w *Worker) GetHashrate() float64 {
	elapsed := time.Since(w.startTime).Seconds()
	if elapsed < 1 {
		return 0
	}
	return float64(w.hashCount.Load()) / elapsed
}

// GetStats returns worker statistics
func (w *Worker) GetStats() WorkerStats {
	return WorkerStats{
		WorkerID:      w.cfg.WorkerID,
		WorkerName:    w.cfg.WorkerName,
		Hashrate:      w.GetHashrate(),
		SharesValid:   w.sharesValid.Load(),
		SharesInvalid: w.sharesInvalid.Load(),
		Uptime:        time.Since(w.startTime),
		Mining:        w.mining.Load(),
		Threads:       len(w.rxVMs),
	}
}

// WorkerStats holds worker statistics
type WorkerStats struct {
	WorkerID      string
	WorkerName    string
	Hashrate      float64
	SharesValid   uint64
	SharesInvalid uint64
	Uptime        time.Duration
	Mining        bool
	Threads       int
}

func (w *Worker) heartbeatLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.cfg.HeartbeatInt)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			// Would send heartbeat to coordinator
			w.logger.Debug("Heartbeat",
				"hashrate", w.GetHashrate(),
				"shares", w.sharesValid.Load(),
			)
		}
	}
}

func (w *Worker) hashrateLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastCount uint64

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			count := w.hashCount.Load()
			rate := float64(count-lastCount) / 10.0
			lastCount = count

			if rate > 0 {
				w.logger.Info("Hashrate",
					"h/s", rate,
					"total_hashes", count,
				)
			}
		}
	}
}

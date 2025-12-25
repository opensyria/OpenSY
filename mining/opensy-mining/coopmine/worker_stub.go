//go:build !cgo || !randomx

// Package coopmine - worker_stub.go provides stub types when RandomX is not available
package coopmine

import (
	"context"
	"errors"
	"log/slog"
)

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	WorkerID        string
	WorkerName      string
	CoordinatorAddr string
	Threads         int
	Logger          *slog.Logger
}

// WorkerStats holds worker statistics
type WorkerStats struct {
	Hashrate      float64
	SharesValid   uint64
	SharesInvalid uint64
	JobsReceived  uint64
}

// Worker is a stub for when RandomX is not available
type Worker struct {
	cfg          WorkerConfig
	logger       *slog.Logger
	OnShareFound func(jobID, nonce, result string)
}

// NewWorker creates a stub worker
func NewWorker(cfg WorkerConfig) *Worker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		cfg:    cfg,
		logger: logger,
	}
}

// Start returns an error since RandomX is not available
func (w *Worker) Start() error {
	return errors.New("worker not available: built without RandomX support (requires CGO)")
}

// Stop does nothing
func (w *Worker) Stop() {}

// ID returns the worker ID
func (w *Worker) ID() string {
	return w.cfg.WorkerID
}

// Name returns the worker name
func (w *Worker) Name() string {
	return w.cfg.WorkerName
}

// SetJob sets the current mining job (stub)
func (w *Worker) SetJob(job *Job) {}

// GetHashrate returns the current hashrate (stub returns 0)
func (w *Worker) GetHashrate() float64 {
	return 0
}

// GetStats returns worker statistics (stub)
func (w *Worker) GetStats() *WorkerStats {
	return &WorkerStats{}
}

// Connect connects to coordinator (stub)
func (w *Worker) Connect(ctx context.Context, addr string) error {
	return errors.New("worker not available: built without RandomX support")
}

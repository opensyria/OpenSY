// Package coopmine - service.go integrates coordinator, pool client, and gRPC server
package coopmine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/opensyria/opensy-mining/coopmine/proto/gen"
)

// ServiceConfig holds service configuration
type ServiceConfig struct {
	// Mode: "coordinator" or "worker"
	Mode string

	// Coordinator settings
	ClusterID        string
	ClusterName      string
	GRPCListenAddr   string
	TargetDifficulty uint64

	// Pool connection (coordinator mode)
	PoolAddr   string
	WalletAddr string
	PoolPass   string

	// Worker settings (worker mode)
	CoordinatorAddr string
	WorkerID        string
	WorkerName      string
	Threads         int

	// Common
	Logger *slog.Logger
}

// DefaultServiceConfig returns default configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		Mode:             "coordinator",
		ClusterID:        "coopmine-1",
		ClusterName:      "CoopMine Cluster",
		GRPCListenAddr:   ":5555",
		TargetDifficulty: 10000,
		PoolPass:         "x",
		Threads:          0, // Auto-detect
		Logger:           slog.Default(),
	}
}

// Service is the main CoopMine service
type Service struct {
	cfg    ServiceConfig
	logger *slog.Logger

	// Coordinator mode components
	coordinator *Coordinator
	poolClient  *PoolClient
	grpcServer  *GRPCServer

	// Worker mode components
	worker     *Worker
	grpcClient *GRPCClient

	// Stats
	startTime time.Time

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new CoopMine service
func NewService(cfg ServiceConfig) *Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		cfg:       cfg,
		logger:    cfg.Logger.With("component", "coopmine_service", "mode", cfg.Mode),
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the CoopMine service
func (s *Service) Start() error {
	s.logger.Info("Starting CoopMine service",
		"mode", s.cfg.Mode,
	)

	switch s.cfg.Mode {
	case "coordinator":
		return s.startCoordinator()
	case "worker":
		return s.startWorker()
	default:
		return fmt.Errorf("unknown mode: %s", s.cfg.Mode)
	}
}

func (s *Service) startCoordinator() error {
	s.logger.Info("Starting in coordinator mode",
		"cluster_id", s.cfg.ClusterID,
		"cluster_name", s.cfg.ClusterName,
		"grpc_addr", s.cfg.GRPCListenAddr,
		"pool_addr", s.cfg.PoolAddr,
	)

	// Create coordinator
	coordCfg := ClusterConfig{
		ClusterID:        s.cfg.ClusterID,
		ClusterName:      s.cfg.ClusterName,
		TargetDifficulty: s.cfg.TargetDifficulty,
		HeartbeatInt:     10 * time.Second,
		WorkerTimeout:    60 * time.Second,
		JobTimeout:       5 * time.Minute,
		Logger:           s.cfg.Logger,
	}
	s.coordinator = NewCoordinator(coordCfg)

	// Start coordinator
	if err := s.coordinator.Start(); err != nil {
		return fmt.Errorf("failed to start coordinator: %w", err)
	}

	// Create and connect pool client
	poolCfg := PoolClientConfig{
		PoolAddr:   s.cfg.PoolAddr,
		WalletAddr: s.cfg.WalletAddr,
		Password:   s.cfg.PoolPass,
		WorkerName: s.cfg.ClusterName,
		RigID:      s.cfg.ClusterID,
		Logger:     s.cfg.Logger,
	}
	s.poolClient = NewPoolClient(poolCfg)

	// Set up job forwarding from pool to coordinator
	s.poolClient.OnJob = func(poolJob *PoolJob) {
		job := &Job{
			ID:       poolJob.JobID,
			Blob:     poolJob.Blob,
			Target:   poolJob.Target,
			SeedHash: poolJob.SeedHash,
			Height:   poolJob.Height,
		}
		s.coordinator.SetJob(job)
	}

	// Connect to pool
	if err := s.poolClient.Start(); err != nil {
		s.coordinator.Stop()
		return fmt.Errorf("failed to connect to pool: %w", err)
	}

	// Create and start gRPC server
	grpcCfg := GRPCServerConfig{
		ListenAddr: s.cfg.GRPCListenAddr,
		Logger:     s.cfg.Logger,
	}
	s.grpcServer = NewGRPCServer(grpcCfg, s.coordinator)

	if err := s.grpcServer.Start(); err != nil {
		s.poolClient.Stop()
		s.coordinator.Stop()
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Set up share forwarding from coordinator to pool
	s.coordinator.OnShareAccepted = func(jobID, nonce, result string) (bool, error) {
		return s.poolClient.Submit(jobID, nonce, result)
	}

	s.logger.Info("Coordinator started successfully",
		"grpc", s.cfg.GRPCListenAddr,
		"pool", s.cfg.PoolAddr,
	)

	return nil
}

func (s *Service) startWorker() error {
	s.logger.Info("Starting in worker mode",
		"coordinator", s.cfg.CoordinatorAddr,
		"worker_id", s.cfg.WorkerID,
		"worker_name", s.cfg.WorkerName,
		"threads", s.cfg.Threads,
	)

	// Create gRPC client
	clientCfg := GRPCClientConfig{
		CoordinatorAddr: s.cfg.CoordinatorAddr,
		WorkerID:        s.cfg.WorkerID,
		WorkerName:      s.cfg.WorkerName,
		Threads:         s.cfg.Threads,
		Logger:          s.cfg.Logger,
		HeartbeatInt:    10 * time.Second,
	}
	s.grpcClient = NewGRPCClient(clientCfg)

	// Create worker
	workerCfg := WorkerConfig{
		WorkerID:        s.cfg.WorkerID,
		WorkerName:      s.cfg.WorkerName,
		CoordinatorAddr: s.cfg.CoordinatorAddr,
		Threads:         s.cfg.Threads,
		Logger:          s.cfg.Logger,
	}
	s.worker = NewWorker(workerCfg)

	// Wire up gRPC client to worker
	s.grpcClient.OnJob = func(job *pb.JobMessage) {
		workerJob := &Job{
			ID:         job.JobId,
			Blob:       fmt.Sprintf("%x", job.Blob),
			Target:     fmt.Sprintf("%x", job.Target),
			SeedHash:   fmt.Sprintf("%x", job.SeedHash),
			Height:     job.Height,
			ExtraNonce: job.ExtraNonce,
		}
		s.worker.SetJob(workerJob)
	}

	// Wire up worker to gRPC client for share submission
	s.worker.OnShareFound = func(jobID, nonce, result string) {
		_, err := s.grpcClient.SubmitShare(jobID, nonce, result)
		if err != nil {
			s.logger.Error("Failed to submit share", "err", err)
		}
	}

	// Connect to coordinator
	if err := s.grpcClient.Start(); err != nil {
		return fmt.Errorf("failed to connect to coordinator: %w", err)
	}

	// Start worker
	if err := s.worker.Start(); err != nil {
		s.grpcClient.Stop()
		return fmt.Errorf("failed to start worker: %w", err)
	}

	// Update hashrate periodically
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.grpcClient.UpdateHashrate(s.worker.GetHashrate())
			}
		}
	}()

	s.logger.Info("Worker started successfully")
	return nil
}

// Stop stops the CoopMine service
func (s *Service) Stop() {
	s.logger.Info("Stopping CoopMine service")
	s.cancel()

	switch s.cfg.Mode {
	case "coordinator":
		s.stopCoordinator()
	case "worker":
		s.stopWorker()
	}

	s.wg.Wait()
	s.logger.Info("CoopMine service stopped")
}

func (s *Service) stopCoordinator() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
	if s.poolClient != nil {
		s.poolClient.Stop()
	}
	if s.coordinator != nil {
		s.coordinator.Stop()
	}
}

func (s *Service) stopWorker() {
	if s.worker != nil {
		s.worker.Stop()
	}
	if s.grpcClient != nil {
		s.grpcClient.Stop()
	}
}

// GetStats returns service statistics
func (s *Service) GetStats() ServiceStats {
	stats := ServiceStats{
		Mode:    s.cfg.Mode,
		Uptime:  time.Since(s.startTime),
		Started: s.startTime,
	}

	switch s.cfg.Mode {
	case "coordinator":
		if s.coordinator != nil {
			clusterStats := s.coordinator.GetStats()
			stats.WorkersOnline = clusterStats.OnlineWorkers
			stats.TotalHashrate = clusterStats.TotalHashrate
			stats.SharesValid = clusterStats.SharesValid
			stats.SharesInvalid = clusterStats.SharesInvalid
			stats.BlocksFound = clusterStats.BlocksFound
		}
		if s.poolClient != nil {
			stats.PoolConnected = s.poolClient.IsConnected()
		}

	case "worker":
		if s.worker != nil {
			workerStats := s.worker.GetStats()
			stats.TotalHashrate = workerStats.Hashrate
			stats.SharesValid = workerStats.SharesValid
			stats.SharesInvalid = workerStats.SharesInvalid
		}
		if s.grpcClient != nil {
			stats.CoordinatorConnected = s.grpcClient.IsConnected()
		}
	}

	return stats
}

// ServiceStats holds service statistics
type ServiceStats struct {
	Mode                 string
	Uptime               time.Duration
	Started              time.Time
	WorkersOnline        int
	TotalHashrate        float64
	SharesValid          uint64
	SharesInvalid        uint64
	BlocksFound          uint64
	PoolConnected        bool
	CoordinatorConnected bool
	Mining               bool
	Threads              int
}

// GetCoordinator returns the coordinator (coordinator mode only)
func (s *Service) GetCoordinator() *Coordinator {
	return s.coordinator
}

// GetWorker returns the worker (worker mode only)
func (s *Service) GetWorker() *Worker {
	return s.worker
}

// GetPoolClient returns the pool client (coordinator mode only)
func (s *Service) GetPoolClient() *PoolClient {
	return s.poolClient
}

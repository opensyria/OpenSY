// Package coopmine - grpc_server.go implements gRPC server using generated proto
package coopmine

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	pb "github.com/opensyria/opensy-mining/coopmine/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// GRPCServerConfig holds gRPC server configuration
type GRPCServerConfig struct {
	ListenAddr string
	TLSCert    string // Path to TLS certificate
	TLSKey     string // Path to TLS private key
	Logger     *slog.Logger
}

// GRPCServer wraps coordinator with gRPC interface
type GRPCServer struct {
	pb.UnimplementedCoopMineServer

	cfg         GRPCServerConfig
	coordinator *Coordinator
	logger      *slog.Logger
	server      *grpc.Server

	// Job streams for workers
	jobStreams   map[string]chan *pb.JobMessage
	jobStreamsMu sync.RWMutex

	// Shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(cfg GRPCServerConfig, coordinator *Coordinator) *GRPCServer {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCServer{
		cfg:         cfg,
		coordinator: coordinator,
		logger:      cfg.Logger.With("component", "grpc_server"),
		jobStreams:  make(map[string]chan *pb.JobMessage),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the gRPC server
func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create gRPC server options
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  30 * time.Second,
			Timeout:               10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Add TLS if configured
	if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
		creds, err := credentials.NewServerTLSFromFile(s.cfg.TLSCert, s.cfg.TLSKey)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
		s.logger.Info("TLS enabled")
	}

	s.server = grpc.NewServer(opts...)
	pb.RegisterCoopMineServer(s.server, s)

	s.logger.Info("Starting gRPC server", "addr", s.cfg.ListenAddr)

	// Start job broadcaster
	go s.jobBroadcaster()

	// Serve
	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.Error("gRPC server error", "err", err)
		}
	}()

	return nil
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() {
	s.cancel()
	if s.server != nil {
		s.server.GracefulStop()
	}
	s.logger.Info("gRPC server stopped")
}

// Register handles worker registration
func (s *GRPCServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// Use worker address or a placeholder if not provided
	workerAddr := fmt.Sprintf("grpc://%s", req.WorkerId)
	_, err := s.coordinator.RegisterWorker(req.WorkerId, req.WorkerName, workerAddr)
	if err != nil {
		return &pb.RegisterResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Create job stream for this worker
	s.jobStreamsMu.Lock()
	s.jobStreams[req.WorkerId] = make(chan *pb.JobMessage, 10)
	s.jobStreamsMu.Unlock()

	// Get current job
	var currentJob *pb.JobMessage
	job, err := s.coordinator.GetJobForWorker(req.WorkerId)
	if err == nil && job != nil {
		currentJob = s.jobToProto(job)
	}

	s.logger.Info("Worker registered",
		"worker_id", req.WorkerId,
		"worker_name", req.WorkerName,
		"threads", req.Threads,
	)

	return &pb.RegisterResponse{
		Success:    true,
		Message:    "Registered successfully",
		AssignedId: req.WorkerId,
		Config: &pb.ClusterConfig{
			ClusterId:           s.coordinator.cfg.ClusterID,
			ClusterName:         s.coordinator.cfg.ClusterName,
			TargetDifficulty:    s.coordinator.cfg.TargetDifficulty,
			HeartbeatIntervalMs: int32(s.coordinator.cfg.HeartbeatInt.Milliseconds()),
			JobTimeoutMs:        int32(s.coordinator.cfg.JobTimeout.Milliseconds()),
		},
		CurrentJob: currentJob,
	}, nil
}

// Unregister handles worker unregistration
func (s *GRPCServer) Unregister(ctx context.Context, req *pb.UnregisterRequest) (*pb.UnregisterResponse, error) {
	s.coordinator.UnregisterWorker(req.WorkerId)

	// Close and remove job stream
	s.jobStreamsMu.Lock()
	if ch, ok := s.jobStreams[req.WorkerId]; ok {
		close(ch)
		delete(s.jobStreams, req.WorkerId)
	}
	s.jobStreamsMu.Unlock()

	s.logger.Info("Worker unregistered", "worker_id", req.WorkerId)

	return &pb.UnregisterResponse{
		Success: true,
		Message: "Unregistered successfully",
	}, nil
}

// Heartbeat handles worker heartbeat
func (s *GRPCServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if err := s.coordinator.Heartbeat(req.WorkerId, req.Hashrate); err != nil {
		return &pb.HeartbeatResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.HeartbeatResponse{
		Success: true,
		Command: pb.WorkerCommand_COMMAND_NONE,
	}, nil
}

// StreamJobs streams jobs to a worker
func (s *GRPCServer) StreamJobs(req *pb.StreamJobsRequest, stream pb.CoopMine_StreamJobsServer) error {
	s.jobStreamsMu.RLock()
	ch, ok := s.jobStreams[req.WorkerId]
	s.jobStreamsMu.RUnlock()

	if !ok {
		return fmt.Errorf("worker not registered: %s", req.WorkerId)
	}

	s.logger.Info("Starting job stream", "worker_id", req.WorkerId)

	for {
		select {
		case <-s.ctx.Done():
			return nil
		case <-stream.Context().Done():
			return stream.Context().Err()
		case job, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(job); err != nil {
				s.logger.Error("Failed to send job", "worker", req.WorkerId, "err", err)
				return err
			}
			s.logger.Debug("Job sent", "worker", req.WorkerId, "job", job.JobId)
		}
	}
}

// SubmitShare handles share submission
func (s *GRPCServer) SubmitShare(ctx context.Context, req *pb.ShareRequest) (*pb.ShareResponse, error) {
	share := &Share{
		WorkerID:  req.WorkerId,
		JobID:     req.JobId,
		Nonce:     hex.EncodeToString(req.Nonce),
		Result:    hex.EncodeToString(req.Result),
		Timestamp: time.Now(),
	}

	accepted, err := s.coordinator.SubmitShare(share)
	var reason string
	if err != nil {
		reason = err.Error()
	}

	s.logger.Info("Share submitted",
		"worker", req.WorkerId,
		"job", req.JobId,
		"accepted", accepted,
	)

	return &pb.ShareResponse{
		Accepted:  accepted,
		Reason:    reason,
		IsBlock:   false,
		BlockHash: "",
	}, nil
}

// GetClusterStats returns cluster statistics
func (s *GRPCServer) GetClusterStats(ctx context.Context, req *pb.ClusterStatsRequest) (*pb.ClusterStatsResponse, error) {
	stats := s.coordinator.GetStats()

	// Get worker summaries
	workers := make([]*pb.WorkerSummary, 0)
	s.coordinator.ForEachWorker(func(w *WorkerInfo) {
		status := "idle"
		switch w.Status {
		case WorkerMining:
			status = "mining"
		case WorkerOffline:
			status = "offline"
		}
		workers = append(workers, &pb.WorkerSummary{
			WorkerId:   w.ID,
			WorkerName: w.Name,
			Status:     status,
			Hashrate:   w.Hashrate,
			Shares:     w.SharesValid,
			LastSeen:   w.LastSeen.Unix(),
		})
	})

	return &pb.ClusterStatsResponse{
		ClusterId:     s.coordinator.GetClusterID(),
		ClusterName:   s.coordinator.GetClusterName(),
		WorkersOnline: int32(stats.OnlineWorkers),
		TotalHashrate: stats.TotalHashrate,
		SharesValid:   stats.SharesValid,
		SharesInvalid: stats.SharesInvalid,
		BlocksFound:   stats.BlocksFound,
		UptimeMs:      stats.Uptime.Milliseconds(),
		Workers:       workers,
	}, nil
}

// GetWorkerStats returns worker statistics
func (s *GRPCServer) GetWorkerStats(ctx context.Context, req *pb.WorkerStatsRequest) (*pb.WorkerStatsResponse, error) {
	worker := s.coordinator.GetWorker(req.WorkerId)
	if worker == nil {
		return nil, fmt.Errorf("worker not found: %s", req.WorkerId)
	}

	status := "idle"
	switch worker.Status {
	case WorkerMining:
		status = "mining"
	case WorkerOffline:
		status = "offline"
	}

	return &pb.WorkerStatsResponse{
		WorkerId:      worker.ID,
		WorkerName:    worker.Name,
		Status:        status,
		Hashrate:      worker.Hashrate,
		SharesValid:   worker.SharesValid,
		SharesInvalid: worker.SharesInvalid,
		Threads:       0, // Thread count not tracked in WorkerInfo
		ConnectedAt:   worker.JoinedAt.Unix(),
		LastShareAt:   worker.LastSeen.Unix(),
		CurrentJobId:  worker.CurrentJob,
	}, nil
}

// jobBroadcaster broadcasts jobs from coordinator to all worker streams
func (s *GRPCServer) jobBroadcaster() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case job := <-s.coordinator.JobChannel():
			s.broadcastJob(job)
		}
	}
}

func (s *GRPCServer) broadcastJob(job *Job) {
	s.jobStreamsMu.RLock()
	defer s.jobStreamsMu.RUnlock()

	for workerID, ch := range s.jobStreams {
		extraNonce := s.coordinator.GetWorkerExtraNonce(workerID)

		msg := s.jobToProtoWithNonce(job, extraNonce)

		select {
		case ch <- msg:
		default:
			s.logger.Warn("Job channel full for worker", "worker", workerID)
		}
	}

	s.logger.Debug("Job broadcast", "job_id", job.ID, "workers", len(s.jobStreams))
}

func (s *GRPCServer) jobToProto(job *Job) *pb.JobMessage {
	return s.jobToProtoWithNonce(job, job.ExtraNonce)
}

func (s *GRPCServer) jobToProtoWithNonce(job *Job, extraNonce uint32) *pb.JobMessage {
	blob, _ := hex.DecodeString(job.Blob)
	target, _ := hex.DecodeString(job.Target)
	seedHash, _ := hex.DecodeString(job.SeedHash)

	return &pb.JobMessage{
		JobId:      job.ID,
		Blob:       blob,
		Target:     target,
		SeedHash:   seedHash,
		Height:     job.Height,
		ExtraNonce: extraNonce,
		Timestamp:  time.Now().Unix(),
		CleanJobs:  true,
	}
}

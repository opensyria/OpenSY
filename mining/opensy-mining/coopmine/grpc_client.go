// Package coopmine - grpc_client.go implements gRPC client using generated proto
package coopmine

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/opensyria/opensy-mining/coopmine/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// GRPCClientConfig holds gRPC client configuration
type GRPCClientConfig struct {
	CoordinatorAddr string
	WorkerID        string
	WorkerName      string
	Threads         int
	UseTLS          bool
	TLSInsecure     bool // Skip TLS verification (for self-signed certs)
	Logger          *slog.Logger
	HeartbeatInt    time.Duration
	ReconnectInt    time.Duration
}

// DefaultGRPCClientConfig returns default configuration
func DefaultGRPCClientConfig() GRPCClientConfig {
	return GRPCClientConfig{
		Threads:      1,
		UseTLS:       false,
		HeartbeatInt: 10 * time.Second,
		ReconnectInt: 5 * time.Second,
		Logger:       slog.Default(),
	}
}

// GRPCClient connects workers to coordinator
type GRPCClient struct {
	cfg    GRPCClientConfig
	logger *slog.Logger

	// gRPC
	conn   *grpc.ClientConn
	client pb.CoopMineClient
	connMu sync.Mutex

	// State
	connected  atomic.Bool
	registered atomic.Bool

	// Cluster config received from coordinator
	clusterConfig *pb.ClusterConfig
	configMu      sync.RWMutex

	// Callbacks
	OnJob          func(job *pb.JobMessage)
	OnCommand      func(cmd pb.WorkerCommand)
	OnConnected    func()
	OnDisconnected func(err error)

	// Stats
	sharesValid   atomic.Uint64
	sharesInvalid atomic.Uint64
	hashrate      atomic.Uint64 // Stored as *1000 to preserve decimals

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewGRPCClient creates a new gRPC client
func NewGRPCClient(cfg GRPCClientConfig) *GRPCClient {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCClient{
		cfg:    cfg,
		logger: cfg.Logger.With("component", "grpc_client", "worker", cfg.WorkerID),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect connects to the coordinator
func (c *GRPCClient) Connect() error {
	c.logger.Info("Connecting to coordinator", "addr", c.cfg.CoordinatorAddr)

	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Add TLS or insecure credentials
	if c.cfg.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.cfg.TLSInsecure,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		c.logger.Info("TLS enabled", "insecure", c.cfg.TLSInsecure)
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(c.cfg.CoordinatorAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.client = pb.NewCoopMineClient(conn)
	c.connMu.Unlock()

	c.connected.Store(true)
	c.logger.Info("Connected to coordinator")

	if c.OnConnected != nil {
		c.OnConnected()
	}

	return nil
}

// Start connects, registers, and starts background tasks
func (c *GRPCClient) Start() error {
	if err := c.Connect(); err != nil {
		return err
	}

	if err := c.Register(); err != nil {
		return err
	}

	// Start heartbeat loop
	c.wg.Add(1)
	go c.heartbeatLoop()

	// Start job stream
	c.wg.Add(1)
	go c.jobStreamLoop()

	return nil
}

// Stop disconnects from coordinator
func (c *GRPCClient) Stop() {
	c.logger.Info("Stopping gRPC client")
	c.cancel()

	// Unregister
	if c.registered.Load() {
		c.Unregister()
	}

	c.wg.Wait()

	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connMu.Unlock()

	c.connected.Store(false)
	c.registered.Store(false)
	c.logger.Info("gRPC client stopped")
}

// Register registers with the coordinator
func (c *GRPCClient) Register() error {
	req := &pb.RegisterRequest{
		WorkerId:   c.cfg.WorkerID,
		WorkerName: c.cfg.WorkerName,
		Threads:    int32(c.cfg.Threads),
		Version:    "1.0.0",
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	c.registered.Store(true)

	if resp.Config != nil {
		c.configMu.Lock()
		c.clusterConfig = resp.Config
		c.configMu.Unlock()
	}

	c.logger.Info("Registered with coordinator",
		"cluster", resp.Config.ClusterName,
		"assigned_id", resp.AssignedId,
	)

	// Process initial job if provided
	if resp.CurrentJob != nil && c.OnJob != nil {
		c.OnJob(resp.CurrentJob)
	}

	return nil
}

// Unregister unregisters from the coordinator
func (c *GRPCClient) Unregister() error {
	req := &pb.UnregisterRequest{
		WorkerId: c.cfg.WorkerID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.Unregister(ctx, req)
	if err != nil {
		c.logger.Warn("Unregister failed", "err", err)
		return err
	}

	c.registered.Store(false)
	c.logger.Info("Unregistered from coordinator")
	return nil
}

// SubmitShare submits a share to the coordinator
func (c *GRPCClient) SubmitShare(jobID, nonce, result string) (*pb.ShareResponse, error) {
	if !c.registered.Load() {
		return nil, fmt.Errorf("not registered")
	}

	nonceBytes, _ := hex.DecodeString(nonce)
	resultBytes, _ := hex.DecodeString(result)

	req := &pb.ShareRequest{
		WorkerId:  c.cfg.WorkerID,
		JobId:     jobID,
		Nonce:     nonceBytes,
		Result:    resultBytes,
		Timestamp: time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.SubmitShare(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Accepted {
		c.sharesValid.Add(1)
	} else {
		c.sharesInvalid.Add(1)
	}

	return resp, nil
}

// UpdateHashrate updates the reported hashrate
func (c *GRPCClient) UpdateHashrate(hashrate float64) {
	c.hashrate.Store(uint64(hashrate * 1000))
}

// GetClusterConfig returns the cluster configuration
func (c *GRPCClient) GetClusterConfig() *pb.ClusterConfig {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.clusterConfig
}

// GetClusterStats fetches cluster statistics from coordinator
func (c *GRPCClient) GetClusterStats() (*pb.ClusterStatsResponse, error) {
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	return c.client.GetClusterStats(ctx, &pb.ClusterStatsRequest{})
}

// IsConnected returns connection status
func (c *GRPCClient) IsConnected() bool {
	return c.connected.Load()
}

// IsRegistered returns registration status
func (c *GRPCClient) IsRegistered() bool {
	return c.registered.Load()
}

func (c *GRPCClient) heartbeatLoop() {
	defer c.wg.Done()

	interval := c.cfg.HeartbeatInt
	c.configMu.RLock()
	if c.clusterConfig != nil && c.clusterConfig.HeartbeatIntervalMs > 0 {
		interval = time.Duration(c.clusterConfig.HeartbeatIntervalMs) * time.Millisecond
	}
	c.configMu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if !c.registered.Load() {
				continue
			}

			hashrate := float64(c.hashrate.Load()) / 1000.0

			req := &pb.HeartbeatRequest{
				WorkerId:      c.cfg.WorkerID,
				Hashrate:      hashrate,
				SharesValid:   c.sharesValid.Load(),
				SharesInvalid: c.sharesInvalid.Load(),
			}

			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			resp, err := c.client.Heartbeat(ctx, req)
			cancel()

			if err != nil {
				c.logger.Warn("Heartbeat failed", "err", err)
				continue
			}

			// Handle commands from coordinator
			if resp.Command != pb.WorkerCommand_COMMAND_NONE && c.OnCommand != nil {
				c.OnCommand(resp.Command)
			}
		}
	}
}

func (c *GRPCClient) jobStreamLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if !c.registered.Load() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		err := c.streamJobs()
		if err != nil && err != io.EOF {
			c.logger.Warn("Job stream error, reconnecting", "err", err)
		}

		// Wait before reconnecting
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(c.cfg.ReconnectInt):
		}
	}
}

func (c *GRPCClient) streamJobs() error {
	c.logger.Info("Starting job stream")

	stream, err := c.client.StreamJobs(c.ctx, &pb.StreamJobsRequest{
		WorkerId: c.cfg.WorkerID,
	})
	if err != nil {
		return err
	}

	for {
		job, err := stream.Recv()
		if err != nil {
			return err
		}

		c.logger.Info("Received job", "job_id", job.JobId, "height", job.Height)

		if c.OnJob != nil {
			c.OnJob(job)
		}
	}
}

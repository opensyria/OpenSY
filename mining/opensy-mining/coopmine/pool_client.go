// Package coopmine - pool_client.go implements upstream connection to mining pool
package coopmine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// PoolClientConfig holds pool client configuration
type PoolClientConfig struct {
	PoolAddr     string
	WalletAddr   string
	Password     string
	WorkerName   string
	RigID        string
	Logger       *slog.Logger
	ReconnectInt time.Duration
}

// DefaultPoolClientConfig returns default configuration
func DefaultPoolClientConfig() PoolClientConfig {
	return PoolClientConfig{
		Password:     "x",
		WorkerName:   "coopmine",
		RigID:        "rig1",
		ReconnectInt: 10 * time.Second,
		Logger:       slog.Default(),
	}
}

// PoolClient connects to upstream mining pool
type PoolClient struct {
	cfg    PoolClientConfig
	logger *slog.Logger

	// Connection
	conn   net.Conn
	connMu sync.Mutex

	// State
	connected atomic.Bool
	loggedIn  atomic.Bool

	// Message handling
	msgID   atomic.Uint64
	pending map[uint64]chan json.RawMessage
	pendMu  sync.Mutex

	// Job handling
	currentJob *PoolJob
	jobMu      sync.RWMutex

	// Callbacks
	OnJob          func(job *PoolJob)
	OnShareResult  func(accepted bool, reason string)
	OnConnected    func()
	OnDisconnected func(err error)

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PoolJob represents a job from the pool
type PoolJob struct {
	JobID    string
	Blob     string
	Target   string
	SeedHash string
	Height   int64
	RecvTime time.Time
}

// Stratum message types
type stratumRequest struct {
	ID     uint64      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type stratumResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *stratumError   `json:"error,omitempty"`
}

type stratumError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type loginParams struct {
	Login string `json:"login"`
	Pass  string `json:"pass"`
	Agent string `json:"agent"`
	RigID string `json:"rigid,omitempty"`
}

type loginResult struct {
	ID         string      `json:"id"`
	Job        *stratumJob `json:"job"`
	Status     string      `json:"status"`
	Extensions []string    `json:"extensions,omitempty"`
}

type stratumJob struct {
	JobID    string `json:"job_id"`
	Blob     string `json:"blob"`
	Target   string `json:"target"`
	SeedHash string `json:"seed_hash"`
	Height   int64  `json:"height"`
}

type submitParams struct {
	ID     string `json:"id"`
	JobID  string `json:"job_id"`
	Nonce  string `json:"nonce"`
	Result string `json:"result"`
}

type submitResult struct {
	Status string `json:"status"`
}

type jobNotify struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  *stratumJob `json:"params"`
}

// NewPoolClient creates a new pool client
func NewPoolClient(cfg PoolClientConfig) *PoolClient {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &PoolClient{
		cfg:     cfg,
		logger:  cfg.Logger.With("component", "pool_client"),
		pending: make(map[uint64]chan json.RawMessage),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Connect connects to the pool
func (pc *PoolClient) Connect() error {
	pc.logger.Info("Connecting to pool", "addr", pc.cfg.PoolAddr)

	conn, err := net.DialTimeout("tcp", pc.cfg.PoolAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	pc.connMu.Lock()
	pc.conn = conn
	pc.connMu.Unlock()

	pc.connected.Store(true)
	pc.logger.Info("Connected to pool")

	if pc.OnConnected != nil {
		pc.OnConnected()
	}

	// Start reader
	pc.wg.Add(1)
	go pc.readLoop()

	return nil
}

// Start connects and logs in
func (pc *PoolClient) Start() error {
	if err := pc.Connect(); err != nil {
		return err
	}

	if err := pc.Login(); err != nil {
		return err
	}

	// Start keepalive
	pc.wg.Add(1)
	go pc.keepaliveLoop()

	return nil
}

// Stop disconnects from the pool
func (pc *PoolClient) Stop() {
	pc.logger.Info("Disconnecting from pool")
	pc.cancel()

	pc.connMu.Lock()
	if pc.conn != nil {
		pc.conn.Close()
	}
	pc.connMu.Unlock()

	pc.wg.Wait()
	pc.connected.Store(false)
	pc.loggedIn.Store(false)
	pc.logger.Info("Disconnected from pool")
}

// Login performs pool login
func (pc *PoolClient) Login() error {
	params := loginParams{
		Login: pc.cfg.WalletAddr + "." + pc.cfg.WorkerName,
		Pass:  pc.cfg.Password,
		Agent: "OpenSY-CoopMine/1.0.0",
		RigID: pc.cfg.RigID,
	}

	result, err := pc.call("login", params)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	var loginRes loginResult
	if err := json.Unmarshal(result, &loginRes); err != nil {
		return fmt.Errorf("failed to parse login result: %w", err)
	}

	pc.loggedIn.Store(true)
	pc.logger.Info("Logged in to pool", "status", loginRes.Status)

	// Process initial job
	if loginRes.Job != nil {
		pc.handleJob(loginRes.Job)
	}

	return nil
}

// Submit submits a share to the pool
func (pc *PoolClient) Submit(jobID, nonce, result string) (bool, error) {
	if !pc.loggedIn.Load() {
		return false, fmt.Errorf("not logged in")
	}

	params := submitParams{
		ID:     "1", // Session ID from login
		JobID:  jobID,
		Nonce:  nonce,
		Result: result,
	}

	res, err := pc.call("submit", params)
	if err != nil {
		if pc.OnShareResult != nil {
			pc.OnShareResult(false, err.Error())
		}
		return false, err
	}

	var submitRes submitResult
	if err := json.Unmarshal(res, &submitRes); err != nil {
		return false, fmt.Errorf("failed to parse submit result: %w", err)
	}

	accepted := submitRes.Status == "OK"
	if pc.OnShareResult != nil {
		pc.OnShareResult(accepted, submitRes.Status)
	}

	pc.logger.Info("Share submitted",
		"job", jobID,
		"accepted", accepted,
	)

	return accepted, nil
}

// GetCurrentJob returns the current job
func (pc *PoolClient) GetCurrentJob() *PoolJob {
	pc.jobMu.RLock()
	defer pc.jobMu.RUnlock()
	return pc.currentJob
}

// IsConnected returns connection status
func (pc *PoolClient) IsConnected() bool {
	return pc.connected.Load()
}

// IsLoggedIn returns login status
func (pc *PoolClient) IsLoggedIn() bool {
	return pc.loggedIn.Load()
}

func (pc *PoolClient) call(method string, params interface{}) (json.RawMessage, error) {
	id := pc.msgID.Add(1)

	req := stratumRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	// Create response channel
	respCh := make(chan json.RawMessage, 1)
	pc.pendMu.Lock()
	pc.pending[id] = respCh
	pc.pendMu.Unlock()

	defer func() {
		pc.pendMu.Lock()
		delete(pc.pending, id)
		pc.pendMu.Unlock()
	}()

	// Send request
	if err := pc.send(req); err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case <-pc.ctx.Done():
		return nil, pc.ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response")
	case result := <-respCh:
		return result, nil
	}
}

func (pc *PoolClient) send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	pc.connMu.Lock()
	defer pc.connMu.Unlock()

	if pc.conn == nil {
		return fmt.Errorf("not connected")
	}

	pc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = pc.conn.Write(data)
	return err
}

func (pc *PoolClient) readLoop() {
	defer pc.wg.Done()
	defer func() {
		pc.connected.Store(false)
		pc.loggedIn.Store(false)
		if pc.OnDisconnected != nil {
			pc.OnDisconnected(nil)
		}
	}()

	pc.connMu.Lock()
	conn := pc.conn
	pc.connMu.Unlock()

	if conn == nil {
		return
	}

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-pc.ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		line, err := reader.ReadBytes('\n')
		if err != nil {
			pc.logger.Error("Read error", "err", err)
			return
		}

		pc.handleMessage(line)
	}
}

func (pc *PoolClient) handleMessage(data []byte) {
	// Try to parse as response (has id)
	var resp stratumResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.ID > 0 {
		pc.pendMu.Lock()
		if ch, ok := pc.pending[resp.ID]; ok {
			if resp.Error != nil {
				pc.logger.Error("RPC error",
					"id", resp.ID,
					"code", resp.Error.Code,
					"msg", resp.Error.Message,
				)
			}
			select {
			case ch <- resp.Result:
			default:
			}
		}
		pc.pendMu.Unlock()
		return
	}

	// Try to parse as job notification
	var notify jobNotify
	if err := json.Unmarshal(data, &notify); err == nil && notify.Method == "job" {
		if notify.Params != nil {
			pc.handleJob(notify.Params)
		}
		return
	}

	pc.logger.Debug("Unknown message", "data", string(data))
}

func (pc *PoolClient) handleJob(job *stratumJob) {
	poolJob := &PoolJob{
		JobID:    job.JobID,
		Blob:     job.Blob,
		Target:   job.Target,
		SeedHash: job.SeedHash,
		Height:   job.Height,
		RecvTime: time.Now(),
	}

	pc.jobMu.Lock()
	pc.currentJob = poolJob
	pc.jobMu.Unlock()

	pc.logger.Info("New job from pool",
		"job_id", job.JobID,
		"height", job.Height,
		"target", job.Target,
	)

	if pc.OnJob != nil {
		pc.OnJob(poolJob)
	}
}

func (pc *PoolClient) keepaliveLoop() {
	defer pc.wg.Done()

	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case <-ticker.C:
			if pc.connected.Load() && pc.loggedIn.Load() {
				if _, err := pc.call("keepalived", nil); err != nil {
					pc.logger.Warn("Keepalive failed", "err", err)
				}
			}
		}
	}
}

// Reconnect attempts to reconnect to the pool
func (pc *PoolClient) Reconnect() error {
	pc.connMu.Lock()
	if pc.conn != nil {
		pc.conn.Close()
	}
	pc.connMu.Unlock()

	pc.connected.Store(false)
	pc.loggedIn.Store(false)

	time.Sleep(pc.cfg.ReconnectInt)

	return pc.Start()
}

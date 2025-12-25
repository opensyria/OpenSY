// Package pool provides the main pool service that coordinates all components.
package pool

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/opensyria/opensy-mining/common/rpc"
	"github.com/opensyria/opensy-mining/pool/cache"
	"github.com/opensyria/opensy-mining/pool/db"
	"github.com/opensyria/opensy-mining/pool/stratum"
)

// Config holds pool service configuration
type Config struct {
	// Stratum
	StratumAddr       string
	InitialDifficulty uint64
	MinDifficulty     uint64
	MaxDifficulty     uint64
	VardiffEnabled    bool

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisAddr     string
	RedisPassword string

	// Node
	NodeURL  string
	NodeUser string
	NodePass string

	// Block confirmation
	ConfirmationDepth int64
	StatsInterval     time.Duration

	Logger *slog.Logger
}

// Service is the main pool service
type Service struct {
	cfg    Config
	logger *slog.Logger

	// Components
	db      *db.DB
	cache   *cache.Cache
	rpc     *rpc.Client
	stratum *stratum.Server
	jobMgr  *stratum.JobManager

	// State
	currentHeight int64
	networkDiff   float64
	mu            sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Stats holds pool statistics
type Stats struct {
	OnlineMiners    int64
	OnlineWorkers   int64
	Hashrate        float64
	BlocksFound     int64
	LastBlockHeight int64
	NetworkDiff     uint64
}

// New creates a new pool service
func New(cfg Config) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		cfg:    cfg,
		logger: cfg.Logger.With("component", "pool-service"),
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize database
	database, err := db.New(db.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		Database: cfg.DBName,
		MaxConns: 20,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	s.db = database
	s.logger.Info("Connected to PostgreSQL")

	// Initialize Redis cache
	redisCache, err := cache.New(cache.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	if err != nil {
		database.Close()
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	s.cache = redisCache
	s.logger.Info("Connected to Redis")

	// Initialize RPC client
	s.rpc = rpc.NewClient(cfg.NodeURL, cfg.NodeUser, cfg.NodePass)
	s.logger.Info("RPC client initialized", "url", cfg.NodeURL)

	// Initialize job manager
	jmCfg := stratum.DefaultJobManagerConfig()
	jmCfg.Logger = cfg.Logger
	s.jobMgr = stratum.NewJobManager(jmCfg, s.rpc)

	// Initialize Stratum server
	stratumCfg := stratum.ServerConfig{
		ListenAddr:        cfg.StratumAddr,
		InitialDifficulty: cfg.InitialDifficulty,
		MinDifficulty:     cfg.MinDifficulty,
		MaxDifficulty:     cfg.MaxDifficulty,
		VardiffEnabled:    cfg.VardiffEnabled,
		Logger:            cfg.Logger,
	}
	s.stratum = stratum.NewServer(stratumCfg, s.jobMgr)

	// Set up Stratum callbacks
	s.stratum.OnMinerConnect = s.handleMinerConnect
	s.stratum.OnMinerDisconnect = s.handleMinerDisconnect
	s.stratum.OnShareSubmit = s.handleShareSubmit
	s.stratum.OnBlockFound = s.handleBlockFound

	return s, nil
}

// Start starts the pool service
func (s *Service) Start() error {
	// Start job manager
	if err := s.jobMgr.Start(); err != nil {
		return fmt.Errorf("failed to start job manager: %w", err)
	}
	s.logger.Info("Job manager started")

	// Start Stratum server
	if err := s.stratum.Start(); err != nil {
		return fmt.Errorf("failed to start Stratum server: %w", err)
	}
	s.logger.Info("Stratum server started", "addr", s.cfg.StratumAddr)

	// Start background loops
	s.wg.Add(3)
	go s.templateRefreshLoop()
	go s.blockConfirmationLoop()
	go s.statsLoop()

	s.logger.Info("Pool service started")
	return nil
}

// Stop stops the pool service gracefully
func (s *Service) Stop() {
	s.logger.Info("Stopping pool service gracefully...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Signal all goroutines to stop
	s.cancel()

	// Stop accepting new connections first
	s.logger.Info("Stopping Stratum server...")
	s.stratum.Stop()

	// Stop job manager
	s.logger.Info("Stopping job manager...")
	s.jobMgr.Stop()

	// Wait for background goroutines with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Background tasks completed")
	case <-shutdownCtx.Done():
		s.logger.Warn("Shutdown timeout, forcing close")
	}

	// Close database connections
	s.logger.Info("Closing database connections...")
	s.db.Close()

	// Close Redis connections
	s.logger.Info("Closing Redis connections...")
	s.cache.Close()

	s.logger.Info("Pool service stopped successfully")
}

// Shutdown performs graceful shutdown handling OS signals
func (s *Service) Shutdown(sig chan os.Signal) {
	<-sig
	s.logger.Info("Received shutdown signal")
	s.Stop()
}

// Stats returns current pool statistics
func (s *Service) Stats() (*Stats, error) {
	ctx := context.Background()

	// Get hashrate from cache
	hashrate, err := s.cache.GetPoolHashrate(ctx, 5)
	if err != nil {
		hashrate = 0
	}

	// Get online counts
	dbStats, err := s.db.GetPoolStats(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	networkDiff := s.networkDiff
	currentHeight := s.currentHeight
	s.mu.RUnlock()

	return &Stats{
		OnlineMiners:    dbStats.OnlineMiners,
		OnlineWorkers:   dbStats.OnlineWorkers,
		Hashrate:        hashrate,
		BlocksFound:     dbStats.TotalBlocks,
		LastBlockHeight: currentHeight,
		NetworkDiff:     uint64(networkDiff),
	}, nil
}

// ActiveMiners returns the count of active miners
func (s *Service) ActiveMiners() int {
	return s.stratum.SessionCount()
}

// Callback handlers

func (s *Service) handleMinerConnect(session *stratum.Session) {
	s.logger.Debug("Miner connected", "session", session.ID, "addr", session.RemoteAddr)
}

func (s *Service) handleMinerDisconnect(session *stratum.Session) {
	s.logger.Debug("Miner disconnected", "session", session.ID)

	// Mark worker offline if we have DB records
	ctx := context.Background()
	sessionData, _ := s.cache.GetSession(ctx, session.ID)
	if sessionData != nil && sessionData.WorkerID > 0 {
		s.db.SetWorkerOffline(ctx, sessionData.WorkerID)
		s.cache.SetWorkerOffline(ctx, sessionData.WorkerID)
	}
	s.cache.DeleteSession(ctx, session.ID)
}

func (s *Service) handleShareSubmit(session *stratum.Session, jobID, nonce, result string, isBlock bool) error {
	ctx := context.Background()

	// Get or create miner/worker records
	miner, err := s.db.GetOrCreateMiner(ctx, session.Login)
	if err != nil {
		return fmt.Errorf("failed to get miner: %w", err)
	}

	worker, err := s.db.GetOrCreateWorker(ctx, miner.ID, session.WorkerName, session.Agent)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// Cache session data for future lookups
	s.cache.SetSession(ctx, session.ID, &cache.SessionData{
		ID:         session.ID,
		MinerID:    miner.ID,
		WorkerID:   worker.ID,
		Login:      session.Login,
		WorkerName: session.WorkerName,
		Difficulty: session.Difficulty,
	}, 30*time.Minute)

	// Record share
	share := &db.Share{
		MinerID:    miner.ID,
		WorkerID:   worker.ID,
		Height:     s.currentHeight,
		Difficulty: session.Difficulty,
		Timestamp:  time.Now(),
		IsValid:    true,
		IsBlock:    isBlock,
	}

	if err := s.db.RecordShare(ctx, share); err != nil {
		s.logger.Error("Failed to record share", "error", err)
	}

	// Record for hashrate calculation
	s.cache.RecordShare(ctx, miner.ID, worker.ID, session.Difficulty)
	s.cache.SetWorkerOnline(ctx, worker.ID)

	return nil
}

func (s *Service) handleBlockFound(session *stratum.Session, height int64, hash string) {
	ctx := context.Background()

	s.logger.Info("BLOCK FOUND!",
		"height", height,
		"hash", hash,
		"miner", session.Login,
		"worker", session.WorkerName,
	)

	// Get miner/worker IDs
	sessionData, _ := s.cache.GetSession(ctx, session.ID)
	if sessionData == nil {
		s.logger.Error("Session data not found for block", "session", session.ID)
		return
	}

	// Get block reward from template (10000 SYL for OpenSY)
	reward := int64(10000_00000000) // 10000 SYL in satoshis

	block := &db.Block{
		Height:     height,
		Hash:       hash,
		MinerID:    sessionData.MinerID,
		WorkerID:   sessionData.WorkerID,
		Reward:     reward,
		Difficulty: s.networkDiff,
		FoundAt:    time.Now(),
	}

	if err := s.db.RecordBlock(ctx, block); err != nil {
		s.logger.Error("Failed to record block", "error", err)
	}
}

// Background loops

func (s *Service) templateRefreshLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.jobMgr.RefreshTemplate(); err != nil {
				s.logger.Error("Failed to refresh template", "error", err)
			}

			// Broadcast to all miners when block changes
			job := s.jobMgr.GetCurrentJob(s.cfg.InitialDifficulty)
			if job != nil {
				s.mu.Lock()
				if job.Height != s.currentHeight {
					s.currentHeight = job.Height
					s.mu.Unlock()
					s.stratum.BroadcastJob()
				} else {
					s.mu.Unlock()
				}
			}
		}
	}
}

func (s *Service) blockConfirmationLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkBlockConfirmations()
		}
	}
}

func (s *Service) checkBlockConfirmations() {
	ctx := context.Background()

	// Get current blockchain height
	blockCount, err := s.rpc.GetBlockCount(ctx)
	if err != nil {
		s.logger.Error("Failed to get block count", "error", err)
		return
	}

	// Get unconfirmed blocks
	blocks, err := s.db.GetUnconfirmedBlocks(ctx)
	if err != nil {
		s.logger.Error("Failed to get unconfirmed blocks", "error", err)
		return
	}

	for _, block := range blocks {
		confirmations := blockCount - block.Height

		if confirmations >= s.cfg.ConfirmationDepth {
			// Verify block is still in main chain
			chainHash, err := s.rpc.GetBlockHash(ctx, block.Height)
			if err != nil {
				s.logger.Error("Failed to get block hash", "height", block.Height, "error", err)
				continue
			}

			if chainHash == block.Hash {
				// Block confirmed
				if err := s.db.ConfirmBlock(ctx, block.ID); err != nil {
					s.logger.Error("Failed to confirm block", "id", block.ID, "error", err)
				} else {
					s.logger.Info("Block confirmed", "height", block.Height, "hash", block.Hash)
				}
			} else {
				// Block orphaned
				if err := s.db.OrphanBlock(ctx, block.ID); err != nil {
					s.logger.Error("Failed to orphan block", "id", block.ID, "error", err)
				} else {
					s.logger.Warn("Block orphaned", "height", block.Height, "hash", block.Hash)
				}
			}
		}
	}
}

func (s *Service) statsLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateStats()
		}
	}
}

func (s *Service) updateStats() {
	ctx := context.Background()

	// Get mining info
	info, err := s.rpc.GetMiningInfo(ctx)
	if err != nil {
		s.logger.Error("Failed to get mining info", "error", err)
		return
	}

	s.mu.Lock()
	s.networkDiff = info.Difficulty
	s.currentHeight = info.Blocks
	s.mu.Unlock()

	// Get pool hashrate
	hashrate, _ := s.cache.GetPoolHashrate(ctx, 5)

	s.logger.Debug("Pool stats",
		"height", info.Blocks,
		"network_diff", info.Difficulty,
		"pool_hashrate", hashrate,
		"miners", s.stratum.SessionCount(),
	)
}

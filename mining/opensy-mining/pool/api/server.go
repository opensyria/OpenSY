// Package api provides the REST API for the mining pool dashboard.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/opensyria/opensy-mining/pool/cache"
	"github.com/opensyria/opensy-mining/pool/db"
)

// Config holds API server configuration
type Config struct {
	ListenAddr string
	Logger     *slog.Logger
}

// Server is the REST API server
type Server struct {
	cfg    Config
	db     *db.DB
	cache  *cache.Cache
	logger *slog.Logger
	server *http.Server
}

// New creates a new API server
func New(cfg Config, database *db.DB, redisCache *cache.Cache) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Server{
		cfg:    cfg,
		db:     database,
		cache:  redisCache,
		logger: cfg.Logger.With("component", "api"),
	}
}

// Start starts the API server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Pool endpoints
	mux.HandleFunc("/api/v1/pool/stats", s.handlePoolStats)
	mux.HandleFunc("/api/v1/pool/blocks", s.handlePoolBlocks)
	mux.HandleFunc("/api/v1/pool/payments", s.handlePoolPayments)

	// Miner endpoints
	mux.HandleFunc("/api/v1/miner/", s.handleMiner)
	mux.HandleFunc("/api/v1/miner/lookup", s.handleMinerLookup)

	// Network endpoints
	mux.HandleFunc("/api/v1/network/stats", s.handleNetworkStats)

	// Health
	mux.HandleFunc("/health", s.handleHealth)

	// CORS middleware
	handler := corsMiddleware(mux)

	s.server = &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.logger.Info("API server starting", "addr", s.cfg.ListenAddr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("API server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the API server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// PoolStatsResponse holds pool statistics
type PoolStatsResponse struct {
	Hashrate       float64 `json:"hashrate"`
	HashrateUnit   string  `json:"hashrate_unit"`
	Miners         int64   `json:"miners"`
	Workers        int64   `json:"workers"`
	BlocksFound    int64   `json:"blocks_found"`
	LastBlockTime  *int64  `json:"last_block_time,omitempty"`
	PoolFee        float64 `json:"pool_fee"`
	MinPayout      string  `json:"min_payout"`
	PayoutInterval string  `json:"payout_interval"`
}

func (s *Server) handlePoolStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hashrate, _ := s.cache.GetPoolHashrate(ctx, 5)

	dbStats, err := s.db.GetPoolStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get pool stats", "error", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	response := PoolStatsResponse{
		Hashrate:       hashrate,
		HashrateUnit:   "H/s",
		Miners:         dbStats.OnlineMiners,
		Workers:        dbStats.OnlineWorkers,
		BlocksFound:    dbStats.TotalBlocks,
		PoolFee:        1.0,
		MinPayout:      "1 SYL",
		PayoutInterval: "1 hour",
	}

	if dbStats.LastBlockTime != nil {
		ts := dbStats.LastBlockTime.Unix()
		response.LastBlockTime = &ts
	}

	jsonResponse(w, http.StatusOK, response)
}

func (s *Server) handlePoolBlocks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	rows, err := s.db.QueryBlocks(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get blocks", "error", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to get blocks")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"blocks": rows,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handlePoolPayments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	rows, err := s.db.QueryPayments(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get payments", "error", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to get payments")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"payments": rows,
		"limit":    limit,
		"offset":   offset,
	})
}

// MinerStatsResponse holds miner statistics
type MinerStatsResponse struct {
	Address        string        `json:"address"`
	Hashrate       float64       `json:"hashrate"`
	HashrateUnit   string        `json:"hashrate_unit"`
	SharesValid    int64         `json:"shares_valid"`
	SharesInvalid  int64         `json:"shares_invalid"`
	BlocksFound    int64         `json:"blocks_found"`
	PendingBalance float64       `json:"pending_balance"`
	TotalPaid      float64       `json:"total_paid"`
	Workers        []WorkerStats `json:"workers"`
}

// WorkerStats holds individual worker statistics
type WorkerStats struct {
	Name     string  `json:"name"`
	Hashrate float64 `json:"hashrate"`
	Shares   int64   `json:"shares"`
	LastSeen int64   `json:"last_seen"`
	Online   bool    `json:"online"`
}

func (s *Server) handleMiner(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	address := path[len("/api/v1/miner/"):]

	if address == "" || address == "lookup" {
		errorResponse(w, http.StatusBadRequest, "Address required")
		return
	}

	ctx := r.Context()

	miner, err := s.db.GetMinerByAddress(ctx, address)
	if err != nil {
		s.logger.Error("Failed to get miner", "error", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to get miner")
		return
	}
	if miner == nil {
		errorResponse(w, http.StatusNotFound, "Miner not found")
		return
	}

	hashrate, _ := s.cache.GetMinerHashrate(ctx, miner.ID, 5)

	workers, err := s.db.GetMinerWorkers(ctx, miner.ID)
	if err != nil {
		s.logger.Error("Failed to get workers", "error", err)
		workers = nil
	}

	workerStats := make([]WorkerStats, 0, len(workers))
	for _, w := range workers {
		workerStats = append(workerStats, WorkerStats{
			Name:     w.Name,
			Hashrate: 0,
			Shares:   w.TotalShares,
			LastSeen: w.LastSeenAt.Unix(),
			Online:   w.IsOnline,
		})
	}

	response := MinerStatsResponse{
		Address:        miner.Address,
		Hashrate:       hashrate,
		HashrateUnit:   "H/s",
		SharesValid:    miner.TotalShares,
		SharesInvalid:  0,
		BlocksFound:    miner.TotalBlocks,
		PendingBalance: float64(miner.PendingPayout) / 100000000.0,
		TotalPaid:      float64(miner.TotalPaid) / 100000000.0,
		Workers:        workerStats,
	}

	jsonResponse(w, http.StatusOK, response)
}

func (s *Server) handleMinerLookup(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		errorResponse(w, http.StatusBadRequest, "address parameter required")
		return
	}

	ctx := r.Context()
	miner, err := s.db.GetMinerByAddress(ctx, address)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Lookup failed")
		return
	}

	if miner == nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"found": false,
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"found":   true,
		"address": miner.Address,
		"shares":  miner.TotalShares,
		"blocks":  miner.TotalBlocks,
	})
}

// NetworkStatsResponse holds network statistics
type NetworkStatsResponse struct {
	Height       int64   `json:"height"`
	Difficulty   float64 `json:"difficulty"`
	NetworkHash  float64 `json:"network_hashrate"`
	BlockTime    int     `json:"block_time"`
	BlockReward  float64 `json:"block_reward"`
	Algo         string  `json:"algorithm"`
	SeedInterval int     `json:"seed_interval"`
}

func (s *Server) handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	response := NetworkStatsResponse{
		Height:       0,
		Difficulty:   0,
		NetworkHash:  0,
		BlockTime:    120,   // 2 minutes
		BlockReward:  10000, // 10000 SYL
		Algo:         "RandomX (rx/0)",
		SeedInterval: 32,
	}

	jsonResponse(w, http.StatusOK, response)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

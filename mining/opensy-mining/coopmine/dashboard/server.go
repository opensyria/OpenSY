// Package dashboard - server.go implements HTTP API and WebSocket for CoopMine dashboard
package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opensyria/opensy-mining/coopmine"
)

// Config holds dashboard configuration
type Config struct {
	ListenAddr     string
	Coordinator    *coopmine.Coordinator
	Logger         *slog.Logger
	StaticDir      string // Path to static files
	UpdateInterval time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		ListenAddr:     ":8080",
		StaticDir:      "./static",
		UpdateInterval: 2 * time.Second,
		Logger:         slog.Default(),
	}
}

// Server is the dashboard HTTP server
type Server struct {
	cfg         Config
	coordinator *coopmine.Coordinator
	logger      *slog.Logger
	server      *http.Server
	upgrader    websocket.Upgrader

	// WebSocket clients
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	broadcast chan interface{}

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new dashboard server
func NewServer(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		cfg:         cfg,
		coordinator: cfg.Coordinator,
		logger:      cfg.Logger.With("component", "dashboard"),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan interface{}, 100),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the dashboard server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/workers", s.handleWorkers)
	mux.HandleFunc("/api/workers/", s.handleWorkerDetail)
	mux.HandleFunc("/api/jobs", s.handleJobs)
	mux.HandleFunc("/api/shares", s.handleShares)
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files
	if s.cfg.StaticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.cfg.StaticDir)))
	}

	s.server = &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start broadcaster
	s.wg.Add(1)
	go s.broadcastLoop()

	// Start stats pusher
	s.wg.Add(1)
	go s.statsPusher()

	s.logger.Info("Starting dashboard server", "addr", s.cfg.ListenAddr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Dashboard server error", "err", err)
		}
	}()

	return nil
}

// Stop stops the dashboard server
func (s *Server) Stop() {
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.server != nil {
		s.server.Shutdown(ctx)
	}

	// Close all WebSocket connections
	s.clientsMu.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clientsMu.Unlock()

	s.wg.Wait()
	s.logger.Info("Dashboard server stopped")
}

// API Handlers

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.getClusterStats()
	writeJSON(w, stats)
}

func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workers := s.getWorkersList()
	writeJSON(w, workers)
}

func (s *Server) handleWorkerDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workerID := r.URL.Path[len("/api/workers/"):]
	if workerID == "" {
		http.Error(w, "Worker ID required", http.StatusBadRequest)
		return
	}

	worker := s.getWorkerStats(workerID)
	if worker == nil {
		http.Error(w, "Worker not found", http.StatusNotFound)
		return
	}

	writeJSON(w, worker)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobs := s.getRecentJobs()
	writeJSON(w, jobs)
}

func (s *Server) handleShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	shares := s.getRecentShares()
	writeJSON(w, shares)
}

// WebSocket handler

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "err", err)
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	s.logger.Info("WebSocket client connected", "remote", conn.RemoteAddr())

	// Send initial stats
	stats := s.getClusterStats()
	conn.WriteJSON(map[string]interface{}{
		"type": "stats",
		"data": stats,
	})

	// Read loop (for pings/pongs)
	go func() {
		defer func() {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			conn.Close()
			s.logger.Info("WebSocket client disconnected", "remote", conn.RemoteAddr())
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}

func (s *Server) broadcastLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.broadcast:
			s.clientsMu.RLock()
			for conn := range s.clients {
				if err := conn.WriteJSON(msg); err != nil {
					s.logger.Debug("WebSocket write failed", "err", err)
				}
			}
			s.clientsMu.RUnlock()
		}
	}
}

func (s *Server) statsPusher() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			stats := s.getClusterStats()
			s.broadcast <- map[string]interface{}{
				"type":      "stats",
				"data":      stats,
				"timestamp": time.Now().Unix(),
			}
		}
	}
}

// Data fetchers

type ClusterStatsResponse struct {
	ClusterID     string        `json:"cluster_id"`
	ClusterName   string        `json:"cluster_name"`
	WorkersOnline int           `json:"workers_online"`
	WorkersTotal  int           `json:"workers_total"`
	TotalHashrate float64       `json:"total_hashrate"`
	HashrateUnit  string        `json:"hashrate_unit"`
	SharesValid   uint64        `json:"shares_valid"`
	SharesInvalid uint64        `json:"shares_invalid"`
	BlocksFound   uint64        `json:"blocks_found"`
	Uptime        int64         `json:"uptime_seconds"`
	LastBlockTime int64         `json:"last_block_time,omitempty"`
	PoolConnected bool          `json:"pool_connected"`
	Workers       []WorkerBrief `json:"workers"`
}

type WorkerBrief struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Hashrate float64 `json:"hashrate"`
	Shares   uint64  `json:"shares"`
	LastSeen int64   `json:"last_seen"`
}

type WorkerDetailResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	Hashrate      float64 `json:"hashrate"`
	HashrateUnit  string  `json:"hashrate_unit"`
	SharesValid   uint64  `json:"shares_valid"`
	SharesInvalid uint64  `json:"shares_invalid"`
	Threads       int     `json:"threads"`
	ConnectedAt   int64   `json:"connected_at"`
	LastSeenAt    int64   `json:"last_seen_at"`
	LastShareAt   int64   `json:"last_share_at"`
	CurrentJobID  string  `json:"current_job_id"`
	ExtraNonce    uint32  `json:"extra_nonce"`
}

func (s *Server) getClusterStats() *ClusterStatsResponse {
	if s.coordinator == nil {
		return &ClusterStatsResponse{}
	}

	stats := s.coordinator.GetStats()

	workers := make([]WorkerBrief, 0)
	s.coordinator.ForEachWorker(func(w *coopmine.WorkerInfo) {
		status := "idle"
		switch w.Status {
		case coopmine.WorkerMining:
			status = "mining"
		case coopmine.WorkerOffline:
			status = "offline"
		}
		workers = append(workers, WorkerBrief{
			ID:       w.ID,
			Name:     w.Name,
			Status:   status,
			Hashrate: w.Hashrate,
			Shares:   w.SharesValid,
			LastSeen: w.LastSeen.Unix(),
		})
	})

	hr, unit := formatHashrateWithUnit(stats.TotalHashrate)

	return &ClusterStatsResponse{
		ClusterID:     s.cfg.Coordinator.GetClusterID(),
		ClusterName:   s.cfg.Coordinator.GetClusterName(),
		WorkersOnline: stats.OnlineWorkers,
		TotalHashrate: hr,
		HashrateUnit:  unit,
		SharesValid:   stats.SharesValid,
		SharesInvalid: stats.SharesInvalid,
		BlocksFound:   stats.BlocksFound,
		Uptime:        int64(stats.Uptime.Seconds()),
		Workers:       workers,
	}
}

func (s *Server) getWorkersList() []WorkerBrief {
	workers := make([]WorkerBrief, 0)

	if s.coordinator == nil {
		return workers
	}

	s.coordinator.ForEachWorker(func(w *coopmine.WorkerInfo) {
		status := "idle"
		switch w.Status {
		case coopmine.WorkerMining:
			status = "mining"
		case coopmine.WorkerOffline:
			status = "offline"
		}
		workers = append(workers, WorkerBrief{
			ID:       w.ID,
			Name:     w.Name,
			Status:   status,
			Hashrate: w.Hashrate,
			Shares:   w.SharesValid,
			LastSeen: w.LastSeen.Unix(),
		})
	})

	return workers
}

func (s *Server) getWorkerStats(workerID string) *WorkerDetailResponse {
	if s.coordinator == nil {
		return nil
	}

	w := s.coordinator.GetWorker(workerID)
	if w == nil {
		return nil
	}

	hr, unit := formatHashrateWithUnit(w.Hashrate)

	status := "idle"
	switch w.Status {
	case coopmine.WorkerMining:
		status = "mining"
	case coopmine.WorkerOffline:
		status = "offline"
	}

	return &WorkerDetailResponse{
		ID:            w.ID,
		Name:          w.Name,
		Status:        status,
		Hashrate:      hr,
		HashrateUnit:  unit,
		SharesValid:   w.SharesValid,
		SharesInvalid: w.SharesInvalid,
		Threads:       0, // Not tracked in WorkerInfo
		ConnectedAt:   w.JoinedAt.Unix(),
		LastSeenAt:    w.LastSeen.Unix(),
		LastShareAt:   w.LastSeen.Unix(), // Approximate
		CurrentJobID:  w.CurrentJob,
		ExtraNonce:    0, // Not tracked in WorkerInfo
	}
}

func (s *Server) getRecentJobs() []map[string]interface{} {
	// Would return recent jobs from coordinator
	return []map[string]interface{}{}
}

func (s *Server) getRecentShares() []map[string]interface{} {
	// Would return recent shares from coordinator
	return []map[string]interface{}{}
}

// Helpers

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func formatHashrateWithUnit(h float64) (float64, string) {
	switch {
	case h >= 1e12:
		return h / 1e12, "TH/s"
	case h >= 1e9:
		return h / 1e9, "GH/s"
	case h >= 1e6:
		return h / 1e6, "MH/s"
	case h >= 1e3:
		return h / 1e3, "KH/s"
	default:
		return h, "H/s"
	}
}

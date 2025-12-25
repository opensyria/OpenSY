// Package stratum - server.go implements the Stratum TCP server
package stratum

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ServerConfig holds Stratum server configuration
type ServerConfig struct {
	ListenAddr        string
	InitialDifficulty uint64
	MinDifficulty     uint64
	MaxDifficulty     uint64
	VardiffEnabled    bool
	VardiffTarget     float64 // Target shares per minute
	VardiffRetarget   time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	Logger            *slog.Logger
}

// DefaultServerConfig returns default configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ListenAddr:        ":3333",
		InitialDifficulty: 10000,
		MinDifficulty:     1000,
		MaxDifficulty:     1000000000,
		VardiffEnabled:    true,
		VardiffTarget:     4.0, // 4 shares per minute
		VardiffRetarget:   30 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      10 * time.Second,
		Logger:            slog.Default(),
	}
}

// Server is the Stratum mining server
type Server struct {
	cfg      ServerConfig
	listener net.Listener
	logger   *slog.Logger

	// Sessions
	sessions   map[string]*Session
	sessionsMu sync.RWMutex

	// Job management
	jobManager *JobManager

	// Callbacks for external integration
	OnMinerConnect    func(s *Session)
	OnMinerDisconnect func(s *Session)
	OnShareSubmit     func(s *Session, jobID, nonce, result string, isBlock bool) error
	OnBlockFound      func(s *Session, height int64, hash string)

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new Stratum server
func NewServer(cfg ServerConfig, jm *JobManager) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		cfg:        cfg,
		logger:     cfg.Logger.With("component", "stratum"),
		sessions:   make(map[string]*Session),
		jobManager: jm,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the Stratum server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.ListenAddr, err)
	}
	s.listener = listener

	s.logger.Info("Stratum server started", "addr", s.cfg.ListenAddr)

	// Start accept loop
	s.wg.Add(1)
	go s.acceptLoop()

	// Start vardiff loop if enabled
	if s.cfg.VardiffEnabled {
		s.wg.Add(1)
		go s.vardiffLoop()
	}

	return nil
}

// Stop stops the Stratum server
func (s *Server) Stop() {
	s.logger.Info("Stopping Stratum server")
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Close all sessions
	s.sessionsMu.Lock()
	for _, session := range s.sessions {
		session.Close()
	}
	s.sessionsMu.Unlock()

	s.wg.Wait()
	s.logger.Info("Stratum server stopped")
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Error("Accept error", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()

	sessionID := uuid.New().String()[:8]
	session := NewSession(sessionID, conn, s)

	// Set up callbacks
	session.OnLogin = s.handleLogin
	session.OnSubmit = s.handleSubmit

	// Register session
	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()

	if s.OnMinerConnect != nil {
		s.OnMinerConnect(session)
	}

	s.logger.Debug("New connection", "session", sessionID, "addr", conn.RemoteAddr())

	// Handle the connection
	s.readLoop(session)

	// Cleanup
	session.Close()

	s.sessionsMu.Lock()
	delete(s.sessions, sessionID)
	s.sessionsMu.Unlock()

	if s.OnMinerDisconnect != nil {
		s.OnMinerDisconnect(session)
	}
}

func (s *Server) readLoop(session *Session) {
	reader := bufio.NewReader(session.Conn)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline
		session.Conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				session.logger.Debug("Read timeout, closing")
			} else {
				session.logger.Debug("Read error", "error", err)
			}
			return
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			session.logger.Warn("Invalid JSON", "error", err)
			continue
		}

		if err := session.HandleRequest(&req); err != nil {
			session.logger.Error("Request handling error", "error", err)
			return
		}
	}
}

func (s *Server) handleLogin(session *Session, login, pass, agent, rigID string) error {
	// Validate wallet address format (basic check)
	if len(login) < 10 {
		return fmt.Errorf("invalid wallet address")
	}
	return nil
}

func (s *Server) handleSubmit(session *Session, jobID, nonce, result string) error {
	// Delegate to job manager for validation
	isBlock, err := s.jobManager.ValidateShare(session, jobID, nonce, result)
	if err != nil {
		return err
	}

	// Call external handler
	if s.OnShareSubmit != nil {
		if err := s.OnShareSubmit(session, jobID, nonce, result, isBlock); err != nil {
			return err
		}
	}

	// Handle block found
	if isBlock {
		job := s.jobManager.GetJob(jobID)
		if job != nil && s.OnBlockFound != nil {
			s.OnBlockFound(session, job.Height, result)
		}
	}

	return nil
}

// BroadcastJob sends a new job to all connected miners
func (s *Server) BroadcastJob() {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	for _, session := range s.sessions {
		if session.State != StateAuthorized {
			continue
		}

		job := s.GetCurrentJob(session.Difficulty)
		if job != nil {
			if err := session.SendJob(job); err != nil {
				session.logger.Error("Failed to send job", "error", err)
			}
		}
	}
}

// GetCurrentJob returns the current job with the specified difficulty
func (s *Server) GetCurrentJob(difficulty uint64) *Job {
	return s.jobManager.GetCurrentJob(difficulty)
}

// GetSession returns a session by ID
func (s *Server) GetSession(id string) *Session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return s.sessions[id]
}

// SessionCount returns the number of active sessions
func (s *Server) SessionCount() int {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return len(s.sessions)
}

// GetAllSessions returns all active sessions
func (s *Server) GetAllSessions() []*Session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (s *Server) vardiffLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.VardiffRetarget)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.adjustDifficulties()
		}
	}
}

func (s *Server) adjustDifficulties() {
	s.sessionsMu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session.State == StateAuthorized {
			sessions = append(sessions, session)
		}
	}
	s.sessionsMu.RUnlock()

	for _, session := range sessions {
		s.adjustSessionDifficulty(session)
	}
}

func (s *Server) adjustSessionDifficulty(session *Session) {
	elapsed := time.Since(session.VardiffStartTime)
	if elapsed < s.cfg.VardiffRetarget {
		return
	}

	shares := session.VardiffShares
	session.VardiffShares = 0
	session.VardiffStartTime = time.Now()

	if shares == 0 {
		return
	}

	// Calculate actual shares per minute
	minutes := elapsed.Minutes()
	if minutes < 0.1 {
		return
	}

	actualRate := float64(shares) / minutes
	targetRate := s.cfg.VardiffTarget

	// Calculate adjustment ratio
	ratio := actualRate / targetRate

	// Apply adjustment with dampening
	session.mu.Lock()
	currentDiff := session.Difficulty
	session.mu.Unlock()

	var newDiff uint64
	if ratio > 1.2 {
		// Too many shares, increase difficulty
		newDiff = uint64(float64(currentDiff) * ratio)
	} else if ratio < 0.8 {
		// Too few shares, decrease difficulty
		newDiff = uint64(float64(currentDiff) * ratio)
	} else {
		// Within acceptable range
		return
	}

	// Apply limits
	if newDiff < s.cfg.MinDifficulty {
		newDiff = s.cfg.MinDifficulty
	}
	if newDiff > s.cfg.MaxDifficulty {
		newDiff = s.cfg.MaxDifficulty
	}

	if newDiff != currentDiff {
		session.SetDifficulty(newDiff)
		session.logger.Info("Difficulty adjusted",
			"old", currentDiff,
			"new", newDiff,
			"rate", actualRate,
		)

		// Send new job with updated difficulty
		job := s.GetCurrentJob(newDiff)
		if job != nil {
			session.SendJob(job)
		}
	}
}

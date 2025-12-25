// Package stratum - session.go handles miner connection state
package stratum

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// SessionState represents the current state of a miner session
type SessionState int

const (
	StateConnected SessionState = iota
	StateSubscribed
	StateAuthorized
	StateDisconnected
)

// Session represents a connected miner session
type Session struct {
	ID         string
	Conn       net.Conn
	RemoteAddr string

	// Miner info
	Login      string // Wallet address
	WorkerName string // Worker identifier
	RigID      string
	Agent      string // Mining software

	// State
	State      SessionState
	Difficulty uint64

	// Jobs
	CurrentJob  *Job
	LastJobTime time.Time

	// Stats
	SharesValid   atomic.Uint64
	SharesInvalid atomic.Uint64
	LastShareTime time.Time
	ConnectedAt   time.Time

	// Vardiff
	VardiffShares    int
	VardiffStartTime time.Time

	// Internal
	mu     sync.RWMutex
	writer *bufio.Writer
	logger *slog.Logger
	server *Server

	// Callbacks
	OnLogin  func(s *Session, login, pass, agent, rigID string) error
	OnSubmit func(s *Session, jobID, nonce, result string) error
}

// NewSession creates a new miner session
func NewSession(id string, conn net.Conn, server *Server) *Session {
	return &Session{
		ID:               id,
		Conn:             conn,
		RemoteAddr:       conn.RemoteAddr().String(),
		State:            StateConnected,
		Difficulty:       server.cfg.InitialDifficulty,
		ConnectedAt:      time.Now(),
		VardiffStartTime: time.Now(),
		writer:           bufio.NewWriter(conn),
		logger:           server.logger.With("session", id, "addr", conn.RemoteAddr()),
		server:           server,
	}
}

// Send sends a JSON-RPC response or notification to the miner
func (s *Session) Send(msg interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	data = append(data, '\n')

	s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	if _, err := s.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	if err := s.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	return nil
}

// SendResponse sends a JSON-RPC response
func (s *Session) SendResponse(id interface{}, result interface{}, err *Error) error {
	return s.Send(&Response{
		ID:     id,
		Result: result,
		Error:  err,
	})
}

// SendJob sends a new job to the miner
func (s *Session) SendJob(job *Job) error {
	s.mu.Lock()
	s.CurrentJob = job
	s.LastJobTime = time.Now()
	s.mu.Unlock()

	return s.Send(&Notification{
		Method: MethodJob,
		Params: job,
	})
}

// SetDifficulty updates the session difficulty and notifies the miner
func (s *Session) SetDifficulty(diff uint64) error {
	s.mu.Lock()
	s.Difficulty = diff
	s.mu.Unlock()

	// Note: In Stratum, we typically send a new job with the new target
	// rather than a separate difficulty notification
	s.logger.Debug("Difficulty updated", "difficulty", diff)

	return nil
}

// HandleRequest processes an incoming JSON-RPC request
func (s *Session) HandleRequest(req *Request) error {
	switch req.Method {
	case MethodLogin:
		return s.handleLogin(req)
	case MethodSubmit:
		return s.handleSubmit(req)
	case MethodKeepAlive:
		return s.handleKeepAlive(req)
	default:
		s.logger.Warn("Unknown method", "method", req.Method)
		return s.SendResponse(req.ID, nil, ErrInvalidRequest)
	}
}

func (s *Session) handleLogin(req *Request) error {
	params, err := ParseLoginParams(req.Params)
	if err != nil {
		s.logger.Warn("Invalid login params", "error", err)
		return s.SendResponse(req.ID, nil, ErrInvalidRequest)
	}

	// Store miner info
	s.mu.Lock()
	s.Login = params.Login
	s.RigID = params.RigID
	s.Agent = params.Agent

	// Parse worker name from login (format: address.worker)
	if idx := findChar(params.Login, '.'); idx > 0 {
		s.Login = params.Login[:idx]
		s.WorkerName = params.Login[idx+1:]
	} else {
		s.WorkerName = params.RigID
		if s.WorkerName == "" {
			s.WorkerName = "default"
		}
	}
	s.mu.Unlock()

	// Call login callback if set
	if s.OnLogin != nil {
		if err := s.OnLogin(s, params.Login, params.Pass, params.Agent, params.RigID); err != nil {
			s.logger.Warn("Login rejected", "error", err)
			return s.SendResponse(req.ID, nil, ErrUnauthorized)
		}
	}

	s.mu.Lock()
	s.State = StateAuthorized
	s.mu.Unlock()

	s.logger.Info("Miner logged in",
		"login", s.Login,
		"worker", s.WorkerName,
		"agent", s.Agent,
	)

	// Get initial job
	job := s.server.GetCurrentJob(s.Difficulty)
	if job == nil {
		return s.SendResponse(req.ID, nil, &Error{Code: -8, Message: "No job available"})
	}

	s.mu.Lock()
	s.CurrentJob = job
	s.LastJobTime = time.Now()
	s.mu.Unlock()

	return s.SendResponse(req.ID, &LoginResult{
		ID:     s.ID,
		Job:    job,
		Status: "OK",
	}, nil)
}

func (s *Session) handleSubmit(req *Request) error {
	if s.State != StateAuthorized {
		return s.SendResponse(req.ID, nil, ErrNotSubscribed)
	}

	params, err := ParseSubmitParams(req.Params)
	if err != nil {
		s.logger.Warn("Invalid submit params", "error", err)
		return s.SendResponse(req.ID, nil, ErrInvalidRequest)
	}

	// Verify session ID
	if params.ID != s.ID {
		return s.SendResponse(req.ID, nil, ErrUnauthorized)
	}

	// Call submit callback
	if s.OnSubmit != nil {
		if err := s.OnSubmit(s, params.JobID, params.Nonce, params.Result); err != nil {
			s.SharesInvalid.Add(1)

			// Map error to appropriate response
			switch err.Error() {
			case "duplicate share":
				return s.SendResponse(req.ID, nil, ErrDuplicateShare)
			case "job not found":
				return s.SendResponse(req.ID, nil, ErrJobNotFound)
			case "low difficulty":
				return s.SendResponse(req.ID, nil, ErrLowDifficulty)
			default:
				return s.SendResponse(req.ID, nil, &Error{Code: -1, Message: err.Error()})
			}
		}
	}

	s.SharesValid.Add(1)
	s.LastShareTime = time.Now()
	s.VardiffShares++

	return s.SendResponse(req.ID, &SubmitResult{Status: "OK"}, nil)
}

func (s *Session) handleKeepAlive(req *Request) error {
	return s.SendResponse(req.ID, &map[string]string{"status": "KEEPALIVED"}, nil)
}

// Close closes the session connection
func (s *Session) Close() {
	s.mu.Lock()
	s.State = StateDisconnected
	s.mu.Unlock()

	s.Conn.Close()
	s.logger.Debug("Session closed")
}

// Stats returns session statistics
func (s *Session) Stats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionStats{
		ID:            s.ID,
		Login:         s.Login,
		WorkerName:    s.WorkerName,
		Agent:         s.Agent,
		Difficulty:    s.Difficulty,
		SharesValid:   s.SharesValid.Load(),
		SharesInvalid: s.SharesInvalid.Load(),
		LastShareTime: s.LastShareTime,
		ConnectedAt:   s.ConnectedAt,
		State:         s.State,
	}
}

// SessionStats holds session statistics for reporting
type SessionStats struct {
	ID            string
	Login         string
	WorkerName    string
	Agent         string
	Difficulty    uint64
	SharesValid   uint64
	SharesInvalid uint64
	LastShareTime time.Time
	ConnectedAt   time.Time
	State         SessionState
}

// Helper function
func findChar(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

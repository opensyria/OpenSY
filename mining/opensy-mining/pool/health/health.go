// Package health provides health check endpoints for the mining pool
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents component health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check is a health check function
type Check func(ctx context.Context) error

// Component represents a monitored component
type Component struct {
	Name      string        `json:"name"`
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Latency   time.Duration `json:"latency_ms"`
	LastCheck time.Time     `json:"last_check"`
}

// Response is the health check response
type Response struct {
	Status     Status                `json:"status"`
	Version    string                `json:"version,omitempty"`
	Uptime     time.Duration         `json:"uptime_seconds"`
	Components map[string]*Component `json:"components"`
	Timestamp  time.Time             `json:"timestamp"`
}

// Handler provides health check HTTP handlers
type Handler struct {
	checks    map[string]Check
	results   map[string]*Component
	mu        sync.RWMutex
	version   string
	startTime time.Time
	interval  time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// Config holds health handler configuration
type Config struct {
	Version       string
	CheckInterval time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		Version:       "unknown",
		CheckInterval: 30 * time.Second,
	}
}

// NewHandler creates a new health handler
func NewHandler(cfg Config) *Handler {
	ctx, cancel := context.WithCancel(context.Background())

	h := &Handler{
		checks:    make(map[string]Check),
		results:   make(map[string]*Component),
		version:   cfg.Version,
		startTime: time.Now(),
		interval:  cfg.CheckInterval,
		ctx:       ctx,
		cancel:    cancel,
	}

	return h
}

// RegisterCheck registers a health check
func (h *Handler) RegisterCheck(name string, check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.checks[name] = check
	h.results[name] = &Component{
		Name:   name,
		Status: StatusHealthy,
	}
}

// Start starts background health checking
func (h *Handler) Start() {
	go h.checkLoop()
}

// Stop stops health checking
func (h *Handler) Stop() {
	h.cancel()
}

func (h *Handler) checkLoop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Run initial check
	h.runChecks()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.runChecks()
		}
	}
}

func (h *Handler) runChecks() {
	h.mu.Lock()
	checks := make(map[string]Check)
	for name, check := range h.checks {
		checks[name] = check
	}
	h.mu.Unlock()

	for name, check := range checks {
		start := time.Now()
		ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		err := check(ctx)
		cancel()
		latency := time.Since(start)

		h.mu.Lock()
		result := h.results[name]
		if result == nil {
			result = &Component{Name: name}
			h.results[name] = result
		}

		result.Latency = latency
		result.LastCheck = time.Now()

		if err != nil {
			result.Status = StatusUnhealthy
			result.Message = err.Error()
		} else {
			result.Status = StatusHealthy
			result.Message = ""
		}
		h.mu.Unlock()
	}
}

// getOverallStatus calculates overall status from components
func (h *Handler) getOverallStatus() Status {
	h.mu.RLock()
	defer h.mu.RUnlock()

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range h.results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// HealthHandler returns the main health check handler
func (h *Handler) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		components := make(map[string]*Component)
		for k, v := range h.results {
			components[k] = v
		}
		h.mu.RUnlock()

		status := h.getOverallStatus()
		response := Response{
			Status:     status,
			Version:    h.version,
			Uptime:     time.Since(h.startTime),
			Components: components,
			Timestamp:  time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		if status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// LivenessHandler returns simple liveness probe (is the process running?)
func (h *Handler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	}
}

// ReadinessHandler returns readiness probe (is the service ready to accept traffic?)
func (h *Handler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := h.getOverallStatus()

		w.Header().Set("Content-Type", "application/json")
		if status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		}
	}
}

// Common health checks

// DatabaseCheck creates a database health check
func DatabaseCheck(pingFn func(context.Context) error) Check {
	return func(ctx context.Context) error {
		return pingFn(ctx)
	}
}

// RedisCheck creates a Redis health check
func RedisCheck(pingFn func(context.Context) error) Check {
	return func(ctx context.Context) error {
		return pingFn(ctx)
	}
}

// RPCCheck creates an RPC node health check
func RPCCheck(getInfoFn func(context.Context) error) Check {
	return func(ctx context.Context) error {
		return getInfoFn(ctx)
	}
}

// StratumCheck creates a Stratum server health check
func StratumCheck(isRunningFn func() bool) Check {
	return func(ctx context.Context) error {
		if !isRunningFn() {
			return ErrStratumNotRunning
		}
		return nil
	}
}

// Custom errors
type healthError string

func (e healthError) Error() string {
	return string(e)
}

var (
	ErrDatabaseUnreachable = healthError("database unreachable")
	ErrRedisUnreachable    = healthError("redis unreachable")
	ErrNodeUnreachable     = healthError("node unreachable")
	ErrStratumNotRunning   = healthError("stratum server not running")
)

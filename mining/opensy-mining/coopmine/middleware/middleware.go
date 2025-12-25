// Package middleware provides production-ready middleware for CoopMine
package middleware

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-client rate limiting
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rateVal  rate.Limit
	burst    int
	cleanup  time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rateVal:  rate.Limit(rps),
		burst:    burst,
		cleanup:  5 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.RLock()
	limiter, exists := rl.limiters[clientIP]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rl.rateVal, rl.burst)
		rl.limiters[clientIP] = limiter
		rl.mu.Unlock()
	}

	return limiter.Allow()
}

// Wait blocks until request is allowed or context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context, clientIP string) error {
	rl.mu.RLock()
	limiter, exists := rl.limiters[clientIP]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rl.rateVal, rl.burst)
		rl.limiters[clientIP] = limiter
		rl.mu.Unlock()
	}

	return limiter.Wait(ctx)
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		if len(rl.limiters) > 10000 {
			rl.limiters = make(map[string]*rate.Limiter)
		}
		rl.mu.Unlock()
	}
}

// ConnectionLimiter limits concurrent connections per client
type ConnectionLimiter struct {
	connections map[string]int
	maxPerIP    int
	maxTotal    int
	total       int
	mu          sync.Mutex
	logger      *slog.Logger
}

// NewConnectionLimiter creates a new connection limiter
func NewConnectionLimiter(maxPerIP, maxTotal int, logger *slog.Logger) *ConnectionLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConnectionLimiter{
		connections: make(map[string]int),
		maxPerIP:    maxPerIP,
		maxTotal:    maxTotal,
		logger:      logger,
	}
}

// Acquire attempts to acquire a connection slot
func (cl *ConnectionLimiter) Acquire(clientIP string) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.total >= cl.maxTotal {
		cl.logger.Warn("Max total connections reached", "total", cl.total, "max", cl.maxTotal)
		return false
	}

	if cl.connections[clientIP] >= cl.maxPerIP {
		cl.logger.Warn("Max connections per IP reached", "ip", clientIP, "current", cl.connections[clientIP], "max", cl.maxPerIP)
		return false
	}

	cl.connections[clientIP]++
	cl.total++
	return true
}

// Release releases a connection slot
func (cl *ConnectionLimiter) Release(clientIP string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.connections[clientIP] > 0 {
		cl.connections[clientIP]--
		cl.total--
	}

	if cl.connections[clientIP] == 0 {
		delete(cl.connections, clientIP)
	}
}

// Stats returns connection statistics
func (cl *ConnectionLimiter) Stats() (total int, byIP map[string]int) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	byIP = make(map[string]int)
	for k, v := range cl.connections {
		byIP[k] = v
	}
	return cl.total, byIP
}

// IPWhitelist manages allowed IP addresses
type IPWhitelist struct {
	ips    map[string]bool
	cidrs  []*net.IPNet
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewIPWhitelist creates a new IP whitelist
func NewIPWhitelist(logger *slog.Logger) *IPWhitelist {
	if logger == nil {
		logger = slog.Default()
	}
	return &IPWhitelist{
		ips:    make(map[string]bool),
		cidrs:  make([]*net.IPNet, 0),
		logger: logger,
	}
}

// AddIP adds an IP address to the whitelist
func (w *IPWhitelist) AddIP(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ips[ip] = true
}

// AddCIDR adds a CIDR range to the whitelist
func (w *IPWhitelist) AddCIDR(cidr string) error {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cidrs = append(w.cidrs, network)
	return nil
}

// Allowed checks if an IP is allowed
func (w *IPWhitelist) Allowed(ipStr string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.ips) == 0 && len(w.cidrs) == 0 {
		return true
	}

	if w.ips[ipStr] {
		return true
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range w.cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name       string
	state      CircuitState
	failures   int
	successes  int
	threshold  int
	resetAfter time.Duration
	lastChange time.Time
	mu         sync.Mutex
	logger     *slog.Logger
}

// CircuitState represents the circuit breaker state
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, threshold int, resetAfter time.Duration, logger *slog.Logger) *CircuitBreaker {
	if logger == nil {
		logger = slog.Default()
	}
	return &CircuitBreaker{
		name:       name,
		state:      CircuitClosed,
		threshold:  threshold,
		resetAfter: resetAfter,
		logger:     logger,
	}
}

// Allow checks if a request is allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastChange) >= cb.resetAfter {
			cb.state = CircuitHalfOpen
			cb.logger.Info("Circuit breaker half-open", "name", cb.name)
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.threshold {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
			cb.logger.Info("Circuit breaker closed", "name", cb.name)
		}
	case CircuitClosed:
		cb.failures = 0
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.lastChange = time.Now()
		cb.logger.Warn("Circuit breaker opened", "name", cb.name)
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.state = CircuitOpen
			cb.lastChange = time.Now()
			cb.logger.Warn("Circuit breaker opened", "name", cb.name, "failures", cb.failures)
		}
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// RequestValidator validates incoming requests
type RequestValidator struct {
	maxJobIDLength    int
	maxWorkerIDLength int
	maxNonceLength    int
	maxResultLength   int
}

// NewRequestValidator creates a new request validator
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		maxJobIDLength:    64,
		maxWorkerIDLength: 64,
		maxNonceLength:    16,
		maxResultLength:   128,
	}
}

// ValidateWorkerID validates a worker ID
func (v *RequestValidator) ValidateWorkerID(id string) error {
	if len(id) == 0 {
		return ErrEmptyWorkerID
	}
	if len(id) > v.maxWorkerIDLength {
		return ErrWorkerIDTooLong
	}
	return nil
}

// ValidateJobID validates a job ID
func (v *RequestValidator) ValidateJobID(id string) error {
	if len(id) == 0 {
		return ErrEmptyJobID
	}
	if len(id) > v.maxJobIDLength {
		return ErrJobIDTooLong
	}
	return nil
}

// ValidateNonce validates a nonce
func (v *RequestValidator) ValidateNonce(nonce string) error {
	if len(nonce) == 0 {
		return ErrEmptyNonce
	}
	if len(nonce) > v.maxNonceLength {
		return ErrNonceTooLong
	}
	return nil
}

// ValidateResult validates a result
func (v *RequestValidator) ValidateResult(result string) error {
	if len(result) == 0 {
		return ErrEmptyResult
	}
	if len(result) > v.maxResultLength {
		return ErrResultTooLong
	}
	return nil
}

// Validation errors
var (
	ErrEmptyWorkerID   = validationError("worker_id is required")
	ErrWorkerIDTooLong = validationError("worker_id exceeds maximum length")
	ErrEmptyJobID      = validationError("job_id is required")
	ErrJobIDTooLong    = validationError("job_id exceeds maximum length")
	ErrEmptyNonce      = validationError("nonce is required")
	ErrNonceTooLong    = validationError("nonce exceeds maximum length")
	ErrEmptyResult     = validationError("result is required")
	ErrResultTooLong   = validationError("result exceeds maximum length")
)

type validationError string

func (e validationError) Error() string {
	return string(e)
}

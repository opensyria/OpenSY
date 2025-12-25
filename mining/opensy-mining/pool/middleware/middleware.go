package middleware

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting per IP
type RateLimiter struct {
	requests map[string]*rateBucket
	mu       sync.Mutex
	limit    int
	window   time.Duration
	logger   *slog.Logger
}

type rateBucket struct {
	count    int
	resetAt  time.Time
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration, logger *slog.Logger) *RateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	rl := &RateLimiter{
		requests: make(map[string]*rateBucket),
		limit:    limit,
		window:   window,
		logger:   logger,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request from IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.requests[ip]

	if !exists || now.After(bucket.resetAt) {
		rl.requests[ip] = &rateBucket{
			count:    1,
			resetAt:  now.Add(rl.window),
			lastSeen: now,
		}
		return true
	}

	bucket.lastSeen = now
	if bucket.count >= rl.limit {
		rl.logger.Warn("Rate limit exceeded", "ip", ip, "count", bucket.count)
		return false
	}
	bucket.count++
	return true
}

// Reset resets the rate limit for an IP
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.requests, ip)
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, bucket := range rl.requests {
			if now.Sub(bucket.lastSeen) > 10*time.Minute {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// IPBanList manages banned IP addresses
type IPBanList struct {
	bans   map[string]*banEntry
	mu     sync.RWMutex
	logger *slog.Logger
}

type banEntry struct {
	Reason    string
	BannedAt  time.Time
	ExpiresAt time.Time
	Permanent bool
}

// NewIPBanList creates a new ban list
func NewIPBanList(logger *slog.Logger) *IPBanList {
	if logger == nil {
		logger = slog.Default()
	}
	bl := &IPBanList{
		bans:   make(map[string]*banEntry),
		logger: logger,
	}
	go bl.cleanupLoop()
	return bl
}

// Ban adds an IP to the ban list
func (bl *IPBanList) Ban(ip, reason string, duration time.Duration) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	entry := &banEntry{
		Reason:    reason,
		BannedAt:  time.Now(),
		Permanent: duration == 0,
	}
	if duration > 0 {
		entry.ExpiresAt = time.Now().Add(duration)
	}
	bl.bans[ip] = entry
	bl.logger.Warn("IP banned", "ip", ip, "reason", reason, "duration", duration)
}

// Unban removes an IP from the ban list
func (bl *IPBanList) Unban(ip string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	delete(bl.bans, ip)
	bl.logger.Info("IP unbanned", "ip", ip)
}

// IsBanned checks if an IP is banned
func (bl *IPBanList) IsBanned(ip string) (bool, string) {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	entry, exists := bl.bans[ip]
	if !exists {
		return false, ""
	}
	if !entry.Permanent && time.Now().After(entry.ExpiresAt) {
		return false, ""
	}
	return true, entry.Reason
}

// ListBans returns all active bans
func (bl *IPBanList) ListBans() map[string]*banEntry {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	result := make(map[string]*banEntry)
	now := time.Now()
	for ip, entry := range bl.bans {
		if entry.Permanent || now.Before(entry.ExpiresAt) {
			result[ip] = entry
		}
	}
	return result
}

func (bl *IPBanList) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		bl.mu.Lock()
		now := time.Now()
		for ip, entry := range bl.bans {
			if !entry.Permanent && now.After(entry.ExpiresAt) {
				delete(bl.bans, ip)
			}
		}
		bl.mu.Unlock()
	}
}

// ConnectionLimiter limits concurrent connections per IP
type ConnectionLimiter struct {
	connections map[string]int
	maxPerIP    int
	maxTotal    int
	total       int
	mu          sync.Mutex
	logger      *slog.Logger
}

// NewConnectionLimiter creates a connection limiter
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

// Acquire tries to acquire a connection slot
func (cl *ConnectionLimiter) Acquire(ip string) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.total >= cl.maxTotal {
		cl.logger.Warn("Max total connections reached", "total", cl.total)
		return false
	}
	if cl.connections[ip] >= cl.maxPerIP {
		cl.logger.Warn("Max connections per IP reached", "ip", ip, "count", cl.connections[ip])
		return false
	}

	cl.connections[ip]++
	cl.total++
	return true
}

// Release releases a connection slot
func (cl *ConnectionLimiter) Release(ip string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.connections[ip] > 0 {
		cl.connections[ip]--
		cl.total--
	}
	if cl.connections[ip] == 0 {
		delete(cl.connections, ip)
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

// ShareValidator validates incoming shares
type ShareValidator struct {
	invalidShares map[string]int // IP -> count of invalid shares
	threshold     int            // auto-ban threshold
	window        time.Duration
	banList       *IPBanList
	mu            sync.Mutex
	logger        *slog.Logger
}

// NewShareValidator creates a share validator
func NewShareValidator(threshold int, window time.Duration, banList *IPBanList, logger *slog.Logger) *ShareValidator {
	if logger == nil {
		logger = slog.Default()
	}
	sv := &ShareValidator{
		invalidShares: make(map[string]int),
		threshold:     threshold,
		window:        window,
		banList:       banList,
		logger:        logger,
	}
	go sv.cleanupLoop()
	return sv
}

// RecordInvalid records an invalid share and may auto-ban
func (sv *ShareValidator) RecordInvalid(ip string) bool {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	sv.invalidShares[ip]++
	count := sv.invalidShares[ip]

	if count >= sv.threshold {
		sv.banList.Ban(ip, "too many invalid shares", 24*time.Hour)
		delete(sv.invalidShares, ip)
		return true // banned
	}
	return false
}

// Reset resets invalid share count for an IP
func (sv *ShareValidator) Reset(ip string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	delete(sv.invalidShares, ip)
}

func (sv *ShareValidator) cleanupLoop() {
	ticker := time.NewTicker(sv.window)
	defer ticker.Stop()
	for range ticker.C {
		sv.mu.Lock()
		sv.invalidShares = make(map[string]int)
		sv.mu.Unlock()
	}
}

// RequestValidator validates request parameters
type RequestValidator struct {
	maxLoginLength  int
	maxWorkerLength int
	maxAgentLength  int
	maxNonceLength  int
}

// NewRequestValidator creates a request validator
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		maxLoginLength:  128,
		maxWorkerLength: 64,
		maxAgentLength:  128,
		maxNonceLength:  16,
	}
}

// Validation errors
var (
	ErrEmptyLogin    = validationError("login is required")
	ErrLoginTooLong  = validationError("login exceeds maximum length")
	ErrWorkerTooLong = validationError("worker name exceeds maximum length")
	ErrAgentTooLong  = validationError("agent exceeds maximum length")
	ErrEmptyNonce    = validationError("nonce is required")
	ErrNonceTooLong  = validationError("nonce exceeds maximum length")
	ErrInvalidNonce  = validationError("nonce contains invalid characters")
)

type validationError string

func (e validationError) Error() string { return string(e) }

// ValidateLogin validates login parameters
func (v *RequestValidator) ValidateLogin(login, worker, agent string) error {
	if len(login) == 0 {
		return ErrEmptyLogin
	}
	if len(login) > v.maxLoginLength {
		return ErrLoginTooLong
	}
	if len(worker) > v.maxWorkerLength {
		return ErrWorkerTooLong
	}
	if len(agent) > v.maxAgentLength {
		return ErrAgentTooLong
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
	// Check hex validity
	for _, c := range nonce {
		if !isHexChar(c) {
			return ErrInvalidNonce
		}
	}
	return nil
}

func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	name          string
	state         CircuitState
	failures      int
	successes     int
	threshold     int
	resetTimeout  time.Duration
	lastStateTime time.Time
	mu            sync.Mutex
	logger        *slog.Logger
}

// CircuitState represents circuit breaker state
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NewCircuitBreaker creates a circuit breaker
func NewCircuitBreaker(name string, threshold int, resetTimeout time.Duration, logger *slog.Logger) *CircuitBreaker {
	if logger == nil {
		logger = slog.Default()
	}
	return &CircuitBreaker{
		name:         name,
		state:        StateClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
		logger:       logger,
	}
}

// ErrCircuitOpen is returned when circuit is open
var ErrCircuitOpen = circuitError("circuit breaker is open")

type circuitError string

func (e circuitError) Error() string { return string(e) }

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.Allow() {
		return ErrCircuitOpen
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}
	cb.RecordSuccess()
	return nil
}

// Allow checks if request is allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastStateTime) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.logger.Info("Circuit breaker half-open", "name", cb.name)
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.threshold {
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
			cb.logger.Info("Circuit breaker closed", "name", cb.name)
		}
	case StateClosed:
		cb.failures = 0
	}
}

// RecordFailure records failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.state = StateOpen
		cb.lastStateTime = time.Now()
		cb.logger.Warn("Circuit breaker opened", "name", cb.name)
	case StateClosed:
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.state = StateOpen
			cb.lastStateTime = time.Now()
			cb.logger.Warn("Circuit breaker opened", "name", cb.name, "failures", cb.failures)
		}
	}
}

// State returns current state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ExtractIP extracts IP from net.Addr
func ExtractIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

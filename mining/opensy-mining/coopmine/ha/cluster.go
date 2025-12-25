// Package ha provides High Availability primitives for CoopMine
package ha

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Role represents the coordinator role
type Role int

const (
	RoleFollower Role = iota
	RoleCandidate
	RoleLeader
)

func (r Role) String() string {
	switch r {
	case RoleFollower:
		return "follower"
	case RoleCandidate:
		return "candidate"
	case RoleLeader:
		return "leader"
	default:
		return "unknown"
	}
}

// ClusterConfig holds cluster configuration
type ClusterConfig struct {
	NodeID            string
	Peers             []string
	ElectionTimeout   time.Duration
	HeartbeatInterval time.Duration
	Logger            *slog.Logger
}

// DefaultClusterConfig returns default configuration
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		ElectionTimeout:   150 * time.Millisecond,
		HeartbeatInterval: 50 * time.Millisecond,
	}
}

// State represents replicated state
type State struct {
	Term     uint64 `json:"term"`
	LeaderID string `json:"leader_id"`
	Data     []byte `json:"data"`
}

// Cluster manages leader election and state replication
type Cluster struct {
	cfg ClusterConfig

	role     Role
	term     uint64
	votedFor string
	leaderID string
	state    *State

	mu     sync.RWMutex
	logger *slog.Logger

	// Channels
	heartbeatC  chan struct{}
	stepDownC   chan struct{}
	roleChangeC chan Role

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Callbacks
	onBecomeLeader   func()
	onBecomeFollower func(leaderID string)
	onStateChange    func(state *State)
}

// NewCluster creates a new cluster node
func NewCluster(cfg ClusterConfig) (*Cluster, error) {
	if cfg.NodeID == "" {
		return nil, errors.New("node ID is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Cluster{
		cfg:         cfg,
		role:        RoleFollower,
		logger:      cfg.Logger.With("component", "cluster", "node", cfg.NodeID),
		heartbeatC:  make(chan struct{}, 1),
		stepDownC:   make(chan struct{}, 1),
		roleChangeC: make(chan Role, 1),
		ctx:         ctx,
		cancel:      cancel,
	}

	return c, nil
}

// OnBecomeLeader sets callback for becoming leader
func (c *Cluster) OnBecomeLeader(fn func()) {
	c.onBecomeLeader = fn
}

// OnBecomeFollower sets callback for becoming follower
func (c *Cluster) OnBecomeFollower(fn func(leaderID string)) {
	c.onBecomeFollower = fn
}

// OnStateChange sets callback for state changes
func (c *Cluster) OnStateChange(fn func(state *State)) {
	c.onStateChange = fn
}

// Start starts the cluster node
func (c *Cluster) Start() {
	c.wg.Add(1)
	go c.run()
	c.logger.Info("Cluster node started", "role", c.role.String())
}

// Stop stops the cluster node
func (c *Cluster) Stop() {
	c.cancel()
	c.wg.Wait()
	c.logger.Info("Cluster node stopped")
}

// Role returns current role
func (c *Cluster) Role() Role {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.role
}

// IsLeader returns true if this node is the leader
func (c *Cluster) IsLeader() bool {
	return c.Role() == RoleLeader
}

// LeaderID returns current leader ID
func (c *Cluster) LeaderID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.leaderID
}

// Term returns current term
func (c *Cluster) Term() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.term
}

// GetState returns current state
func (c *Cluster) GetState() *State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state == nil {
		return nil
	}
	// Return copy
	stateCopy := *c.state
	return &stateCopy
}

// ProposeState proposes new state (leader only)
func (c *Cluster) ProposeState(data []byte) error {
	if !c.IsLeader() {
		return errors.New("not leader")
	}

	c.mu.Lock()
	c.state = &State{
		Term:     c.term,
		LeaderID: c.cfg.NodeID,
		Data:     data,
	}
	state := c.state
	c.mu.Unlock()

	// Notify state change
	if c.onStateChange != nil {
		c.onStateChange(state)
	}

	// In production: replicate to followers
	c.logger.Debug("State proposed", "term", state.Term)

	return nil
}

func (c *Cluster) run() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		switch c.Role() {
		case RoleFollower:
			c.runFollower()
		case RoleCandidate:
			c.runCandidate()
		case RoleLeader:
			c.runLeader()
		}
	}
}

func (c *Cluster) runFollower() {
	timeout := c.randomElectionTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.heartbeatC:
			// Received heartbeat, reset timer
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(c.randomElectionTimeout())
		case <-timer.C:
			// Election timeout, become candidate
			c.becomeCandidate()
			return
		}
	}
}

func (c *Cluster) runCandidate() {
	c.mu.Lock()
	c.term++
	c.votedFor = c.cfg.NodeID
	currentTerm := c.term
	c.mu.Unlock()

	c.logger.Info("Starting election", "term", currentTerm)

	// In simplified mode (no peers), become leader immediately
	if len(c.cfg.Peers) == 0 {
		c.becomeLeader()
		return
	}

	// Request votes from peers (simplified - in production use RPC)
	votes := 1 // Vote for self
	needed := (len(c.cfg.Peers)+1)/2 + 1

	// Simulate election (in production: send RequestVote RPCs)
	timeout := c.randomElectionTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-c.ctx.Done():
		return
	case <-timer.C:
		// Timeout, check if we have enough votes
		if votes >= needed {
			c.becomeLeader()
		} else {
			c.becomeFollower("")
		}
	case <-c.stepDownC:
		c.becomeFollower("")
	}
}

func (c *Cluster) runLeader() {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()

	// Send initial heartbeat
	c.sendHeartbeat()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.sendHeartbeat()
		case <-c.stepDownC:
			c.becomeFollower("")
			return
		}
	}
}

func (c *Cluster) becomeFollower(leaderID string) {
	c.mu.Lock()
	oldRole := c.role
	c.role = RoleFollower
	c.leaderID = leaderID
	c.mu.Unlock()

	if oldRole != RoleFollower {
		c.logger.Info("Became follower", "leader", leaderID)
		if c.onBecomeFollower != nil {
			c.onBecomeFollower(leaderID)
		}
	}
}

func (c *Cluster) becomeCandidate() {
	c.mu.Lock()
	c.role = RoleCandidate
	c.leaderID = ""
	c.mu.Unlock()

	c.logger.Debug("Became candidate")
}

func (c *Cluster) becomeLeader() {
	c.mu.Lock()
	c.role = RoleLeader
	c.leaderID = c.cfg.NodeID
	c.mu.Unlock()

	c.logger.Info("Became leader", "term", c.Term())

	if c.onBecomeLeader != nil {
		c.onBecomeLeader()
	}
}

func (c *Cluster) sendHeartbeat() {
	// In production: send AppendEntries RPCs to all peers
	c.logger.Debug("Sending heartbeat", "term", c.Term())
}

func (c *Cluster) randomElectionTimeout() time.Duration {
	// Return election timeout with jitter
	base := c.cfg.ElectionTimeout
	// Add up to 50% jitter
	jitter := time.Duration(float64(base) * 0.5 * float64(time.Now().UnixNano()%1000) / 1000)
	return base + jitter
}

// HandleHeartbeat handles incoming heartbeat from leader
func (c *Cluster) HandleHeartbeat(leaderID string, term uint64) {
	c.mu.Lock()
	if term >= c.term {
		c.term = term
		c.leaderID = leaderID
		if c.role != RoleFollower {
			c.mu.Unlock()
			c.becomeFollower(leaderID)
			return
		}
	}
	c.mu.Unlock()

	// Signal heartbeat received
	select {
	case c.heartbeatC <- struct{}{}:
	default:
	}
}

// HandleStateUpdate handles state update from leader
func (c *Cluster) HandleStateUpdate(stateJSON []byte) error {
	var state State
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return err
	}

	c.mu.Lock()
	if state.Term >= c.term {
		c.state = &state
		c.term = state.Term
	}
	stateCopy := c.state
	c.mu.Unlock()

	if c.onStateChange != nil && stateCopy != nil {
		c.onStateChange(stateCopy)
	}

	return nil
}

// RoleChangeChannel returns channel for role changes
func (c *Cluster) RoleChangeChannel() <-chan Role {
	return c.roleChangeC
}

// Stats returns cluster statistics
type ClusterStats struct {
	NodeID   string `json:"node_id"`
	Role     string `json:"role"`
	Term     uint64 `json:"term"`
	LeaderID string `json:"leader_id"`
	Peers    int    `json:"peers"`
}

func (c *Cluster) Stats() ClusterStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ClusterStats{
		NodeID:   c.cfg.NodeID,
		Role:     c.role.String(),
		Term:     c.term,
		LeaderID: c.leaderID,
		Peers:    len(c.cfg.Peers),
	}
}

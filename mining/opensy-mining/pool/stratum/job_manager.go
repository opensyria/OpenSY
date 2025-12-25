// Package stratum - job_manager.go handles block templates and job creation
package stratum

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/opensyria/opensy-mining/common/randomx"
	"github.com/opensyria/opensy-mining/common/rpc"
)

// JobManagerConfig holds job manager configuration
type JobManagerConfig struct {
	SeedInterval    int64         // Blocks between RandomX seed changes (32 for OpenSY)
	TemplateRefresh time.Duration // How often to refresh block template
	Logger          *slog.Logger
}

// DefaultJobManagerConfig returns default configuration
func DefaultJobManagerConfig() JobManagerConfig {
	return JobManagerConfig{
		SeedInterval:    32, // OpenSY uses 32-block seed interval
		TemplateRefresh: time.Second,
		Logger:          slog.Default(),
	}
}

// JobManager manages mining jobs and validates shares
type JobManager struct {
	cfg    JobManagerConfig
	rpc    *rpc.Client
	logger *slog.Logger

	// Current template
	template   *rpc.BlockTemplate
	templateMu sync.RWMutex

	// Jobs
	jobs   map[string]*JobData // jobID -> job data
	jobsMu sync.RWMutex

	// RandomX context
	rxCtx    *randomx.Context
	rxMu     sync.Mutex
	seedHash string

	// Submitted share tracking (for duplicate detection)
	submittedShares   map[string]struct{}
	submittedSharesMu sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// JobData holds additional job metadata
type JobData struct {
	Job         *Job
	Template    *rpc.BlockTemplate
	HeaderBlob  []byte // Raw header for RandomX hashing
	CreatedAt   time.Time
	TargetValue uint64 // Target as uint64 for comparison
}

// NewJobManager creates a new job manager
func NewJobManager(cfg JobManagerConfig, rpcClient *rpc.Client) *JobManager {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &JobManager{
		cfg:             cfg,
		rpc:             rpcClient,
		logger:          cfg.Logger.With("component", "job-manager"),
		jobs:            make(map[string]*JobData),
		submittedShares: make(map[string]struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start starts the job manager
func (jm *JobManager) Start() error {
	// Initial template fetch
	if err := jm.RefreshTemplate(); err != nil {
		return fmt.Errorf("failed to get initial template: %w", err)
	}

	// Start template refresh loop
	jm.wg.Add(1)
	go jm.refreshLoop()

	return nil
}

// Stop stops the job manager
func (jm *JobManager) Stop() {
	jm.cancel()
	jm.wg.Wait()

	// Cleanup RandomX
	jm.rxMu.Lock()
	if jm.rxCtx != nil {
		jm.rxCtx.Close()
		jm.rxCtx = nil
	}
	jm.rxMu.Unlock()
}

func (jm *JobManager) refreshLoop() {
	defer jm.wg.Done()

	ticker := time.NewTicker(jm.cfg.TemplateRefresh)
	defer ticker.Stop()

	for {
		select {
		case <-jm.ctx.Done():
			return
		case <-ticker.C:
			if err := jm.RefreshTemplate(); err != nil {
				jm.logger.Error("Failed to refresh template", "error", err)
			}
		}
	}
}

// RefreshTemplate fetches a new block template from the node
func (jm *JobManager) RefreshTemplate() error {
	template, err := jm.rpc.GetBlockTemplate(jm.ctx)
	if err != nil {
		return err
	}

	jm.templateMu.Lock()
	oldHeight := int64(0)
	if jm.template != nil {
		oldHeight = jm.template.Height
	}
	jm.template = template
	jm.templateMu.Unlock()

	// Check if seed changed
	if template.SeedHash != jm.seedHash {
		jm.logger.Info("RandomX seed changed",
			"old", jm.seedHash,
			"new", template.SeedHash,
		)
		if err := jm.updateRandomXSeed(template.SeedHash); err != nil {
			jm.logger.Error("Failed to update RandomX seed", "error", err)
		}
	}

	// Log new block
	if template.Height != oldHeight {
		jm.logger.Info("New block template",
			"height", template.Height,
			"txs", len(template.Transactions),
			"reward", template.CoinbaseValue,
		)

		// Clean old jobs when block changes
		jm.cleanOldJobs()
	}

	return nil
}

func (jm *JobManager) updateRandomXSeed(seedHash string) error {
	seedBytes, err := hex.DecodeString(seedHash)
	if err != nil {
		return fmt.Errorf("invalid seed hash: %w", err)
	}

	jm.rxMu.Lock()
	defer jm.rxMu.Unlock()

	// Close old context
	if jm.rxCtx != nil {
		jm.rxCtx.Close()
		jm.rxCtx = nil
	}

	// Create new context with recommended flags
	ctx, err := randomx.NewContext(randomx.FlagDefault)
	if err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}

	// Initialize cache with seed
	if err := ctx.InitCache(seedBytes); err != nil {
		ctx.Close()
		return fmt.Errorf("failed to init cache: %w", err)
	}

	jm.rxCtx = ctx
	jm.seedHash = seedHash

	jm.logger.Info("RandomX context updated", "seed", seedHash[:16]+"...")
	return nil
}

// GetCurrentJob returns the current job with specified difficulty
func (jm *JobManager) GetCurrentJob(difficulty uint64) *Job {
	jm.templateMu.RLock()
	template := jm.template
	jm.templateMu.RUnlock()

	if template == nil {
		return nil
	}

	return jm.createJob(template, difficulty)
}

func (jm *JobManager) createJob(template *rpc.BlockTemplate, difficulty uint64) *Job {
	// Generate unique job ID
	jobID := jm.generateJobID()

	// Create block header blob for mining
	headerBlob := jm.buildHeaderBlob(template)

	job := &Job{
		JobID:    jobID,
		Blob:     hex.EncodeToString(headerBlob),
		Target:   DifficultyToCompact(difficulty),
		Height:   template.Height,
		SeedHash: template.SeedHash,
		Algo:     "rx/0",
	}

	// Store job data
	jobData := &JobData{
		Job:         job,
		Template:    template,
		HeaderBlob:  headerBlob,
		CreatedAt:   time.Now(),
		TargetValue: difficulty,
	}

	jm.jobsMu.Lock()
	jm.jobs[jobID] = jobData
	jm.jobsMu.Unlock()

	return job
}

func (jm *JobManager) generateJobID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (jm *JobManager) buildHeaderBlob(template *rpc.BlockTemplate) []byte {
	// Build 80-byte block header
	// This follows Bitcoin-style format:
	// - Version: 4 bytes (little-endian)
	// - Previous block hash: 32 bytes
	// - Merkle root: 32 bytes
	// - Timestamp: 4 bytes (little-endian)
	// - Bits: 4 bytes
	// - Nonce: 4 bytes (placeholder, miner fills this)

	header := make([]byte, 80)

	// Version
	binary.LittleEndian.PutUint32(header[0:4], uint32(template.Version))

	// Previous block hash (reversed for internal byte order)
	prevHash, _ := hex.DecodeString(template.PreviousBlockHash)
	for i := 0; i < 32; i++ {
		header[4+i] = prevHash[31-i]
	}

	// Merkle root would be calculated from transactions
	// For now, use a placeholder - real implementation needs coinbase + tx merkle
	merkleRoot := jm.calculateMerkleRoot(template)
	copy(header[36:68], merkleRoot)

	// Timestamp
	binary.LittleEndian.PutUint32(header[68:72], uint32(template.CurTime))

	// Bits
	bits, _ := hex.DecodeString(template.Bits)
	copy(header[72:76], bits)

	// Nonce (placeholder - miner fills this)
	binary.LittleEndian.PutUint32(header[76:80], 0)

	return header
}

func (jm *JobManager) calculateMerkleRoot(template *rpc.BlockTemplate) []byte {
	// Simplified: in production, build proper coinbase and merkle tree
	// For now, return a hash of the coinbase value and height
	root := make([]byte, 32)
	binary.LittleEndian.PutUint64(root[0:8], uint64(template.CoinbaseValue))
	binary.LittleEndian.PutUint64(root[8:16], uint64(template.Height))
	return root
}

// GetJob returns a job by ID
func (jm *JobManager) GetJob(jobID string) *Job {
	jm.jobsMu.RLock()
	defer jm.jobsMu.RUnlock()

	if data, ok := jm.jobs[jobID]; ok {
		return data.Job
	}
	return nil
}

// ValidateShare validates a submitted share
func (jm *JobManager) ValidateShare(session *Session, jobID, nonce, result string) (bool, error) {
	// Get job
	jm.jobsMu.RLock()
	jobData, ok := jm.jobs[jobID]
	jm.jobsMu.RUnlock()

	if !ok {
		return false, fmt.Errorf("job not found")
	}

	// Check for duplicate
	shareKey := fmt.Sprintf("%s:%s:%s", session.ID, jobID, nonce)
	jm.submittedSharesMu.Lock()
	if _, exists := jm.submittedShares[shareKey]; exists {
		jm.submittedSharesMu.Unlock()
		return false, fmt.Errorf("duplicate share")
	}
	jm.submittedShares[shareKey] = struct{}{}
	jm.submittedSharesMu.Unlock()

	// Decode result hash
	resultHash, err := hex.DecodeString(result)
	if err != nil || len(resultHash) != 32 {
		return false, fmt.Errorf("invalid result hash")
	}

	// Decode nonce
	nonceBytes, err := hex.DecodeString(nonce)
	if err != nil || len(nonceBytes) != 4 {
		return false, fmt.Errorf("invalid nonce")
	}

	// Verify the hash
	header := make([]byte, len(jobData.HeaderBlob))
	copy(header, jobData.HeaderBlob)
	copy(header[76:80], nonceBytes) // Insert nonce

	jm.rxMu.Lock()
	if jm.rxCtx == nil {
		jm.rxMu.Unlock()
		return false, fmt.Errorf("RandomX not initialized")
	}
	computedHash, err := jm.rxCtx.CalculateHash(header)
	jm.rxMu.Unlock()
	if err != nil {
		return false, fmt.Errorf("hash calculation failed: %w", err)
	}

	// Verify result matches
	if hex.EncodeToString(computedHash[:]) != result {
		return false, fmt.Errorf("invalid hash")
	}

	// Check if meets share difficulty
	if !HashMeetsDifficulty(computedHash[:], session.Difficulty) {
		return false, fmt.Errorf("low difficulty")
	}

	// Check if meets network difficulty (block found!)
	jm.templateMu.RLock()
	template := jm.template
	jm.templateMu.RUnlock()

	isBlock := false
	if template != nil {
		// Parse network target
		networkTarget, _ := hex.DecodeString(template.Target)
		if len(networkTarget) == 32 {
			// Compare hash to network target
			isBlock = true
			for i := 31; i >= 0; i-- {
				if computedHash[i] > networkTarget[31-i] {
					isBlock = false
					break
				} else if computedHash[i] < networkTarget[31-i] {
					break
				}
			}
		}
	}

	if isBlock {
		jm.logger.Info("BLOCK FOUND!",
			"height", jobData.Job.Height,
			"hash", result,
			"miner", session.Login,
		)
	}

	return isBlock, nil
}

func (jm *JobManager) cleanOldJobs() {
	jm.jobsMu.Lock()
	defer jm.jobsMu.Unlock()

	// Keep jobs from last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	for id, job := range jm.jobs {
		if job.CreatedAt.Before(cutoff) {
			delete(jm.jobs, id)
		}
	}

	// Also clean old shares
	jm.submittedSharesMu.Lock()
	jm.submittedShares = make(map[string]struct{}) // Simple reset on new block
	jm.submittedSharesMu.Unlock()
}

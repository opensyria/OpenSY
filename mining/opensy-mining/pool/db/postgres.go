// Package db provides database access for the mining pool.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	MaxConns int32
}

// DB wraps the PostgreSQL connection pool
type DB struct {
	pool *pgxpool.Pool
}

// New creates a new database connection pool
func New(cfg Config) (*DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable pool_max_conns=%d",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.MaxConns,
	)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	db.pool.Close()
}

// Miner represents a miner account
type Miner struct {
	ID            int64
	Address       string
	Email         string
	CreatedAt     time.Time
	LastSeenAt    time.Time
	TotalShares   int64
	TotalBlocks   int64
	PendingPayout int64
	TotalPaid     int64
}

// Worker represents a mining worker
type Worker struct {
	ID          int64
	MinerID     int64
	Name        string
	Agent       string
	Difficulty  uint64
	CreatedAt   time.Time
	LastSeenAt  time.Time
	TotalShares int64
	IsOnline    bool
}

// Share represents a submitted share
type Share struct {
	ID         int64
	MinerID    int64
	WorkerID   int64
	Height     int64
	Difficulty uint64
	Timestamp  time.Time
	IsValid    bool
	IsBlock    bool
}

// Block represents a found block
type Block struct {
	ID          int64
	Height      int64
	Hash        string
	MinerID     int64
	WorkerID    int64
	Reward      int64
	Difficulty  float64
	FoundAt     time.Time
	Confirmed   bool
	ConfirmedAt *time.Time
	Orphaned    bool
}

// GetOrCreateMiner gets or creates a miner by address
func (db *DB) GetOrCreateMiner(ctx context.Context, address string) (*Miner, error) {
	var miner Miner

	err := db.pool.QueryRow(ctx, `
		INSERT INTO miners (address, created_at, last_seen_at)
		VALUES ($1, NOW(), NOW())
		ON CONFLICT (address) DO UPDATE SET last_seen_at = NOW()
		RETURNING id, address, email, created_at, last_seen_at, total_shares, total_blocks, pending_payout, total_paid
	`, address).Scan(
		&miner.ID, &miner.Address, &miner.Email, &miner.CreatedAt, &miner.LastSeenAt,
		&miner.TotalShares, &miner.TotalBlocks, &miner.PendingPayout, &miner.TotalPaid,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get/create miner: %w", err)
	}

	return &miner, nil
}

// GetOrCreateWorker gets or creates a worker for a miner
func (db *DB) GetOrCreateWorker(ctx context.Context, minerID int64, name, agent string) (*Worker, error) {
	var worker Worker

	err := db.pool.QueryRow(ctx, `
		INSERT INTO workers (miner_id, name, agent, created_at, last_seen_at, is_online)
		VALUES ($1, $2, $3, NOW(), NOW(), true)
		ON CONFLICT (miner_id, name) DO UPDATE SET 
			last_seen_at = NOW(),
			agent = EXCLUDED.agent,
			is_online = true
		RETURNING id, miner_id, name, agent, difficulty, created_at, last_seen_at, total_shares, is_online
	`, minerID, name, agent).Scan(
		&worker.ID, &worker.MinerID, &worker.Name, &worker.Agent, &worker.Difficulty,
		&worker.CreatedAt, &worker.LastSeenAt, &worker.TotalShares, &worker.IsOnline,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get/create worker: %w", err)
	}

	return &worker, nil
}

// RecordShare records a submitted share
func (db *DB) RecordShare(ctx context.Context, share *Share) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert share
	_, err = tx.Exec(ctx, `
		INSERT INTO shares (miner_id, worker_id, height, difficulty, timestamp, is_valid, is_block)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, share.MinerID, share.WorkerID, share.Height, share.Difficulty, share.Timestamp, share.IsValid, share.IsBlock)

	if err != nil {
		return fmt.Errorf("failed to insert share: %w", err)
	}

	// Update miner stats
	_, err = tx.Exec(ctx, `
		UPDATE miners SET 
			total_shares = total_shares + 1,
			last_seen_at = NOW()
		WHERE id = $1
	`, share.MinerID)

	if err != nil {
		return fmt.Errorf("failed to update miner: %w", err)
	}

	// Update worker stats
	_, err = tx.Exec(ctx, `
		UPDATE workers SET 
			total_shares = total_shares + 1,
			last_seen_at = NOW()
		WHERE id = $1
	`, share.WorkerID)

	if err != nil {
		return fmt.Errorf("failed to update worker: %w", err)
	}

	return tx.Commit(ctx)
}

// RecordBlock records a found block
func (db *DB) RecordBlock(ctx context.Context, block *Block) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert block
	err = tx.QueryRow(ctx, `
		INSERT INTO blocks (height, hash, miner_id, worker_id, reward, difficulty, found_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, block.Height, block.Hash, block.MinerID, block.WorkerID, block.Reward, block.Difficulty, block.FoundAt).Scan(&block.ID)

	if err != nil {
		return fmt.Errorf("failed to insert block: %w", err)
	}

	// Update miner block count
	_, err = tx.Exec(ctx, `
		UPDATE miners SET total_blocks = total_blocks + 1 WHERE id = $1
	`, block.MinerID)

	if err != nil {
		return fmt.Errorf("failed to update miner blocks: %w", err)
	}

	return tx.Commit(ctx)
}

// GetUnconfirmedBlocks returns blocks pending confirmation
func (db *DB) GetUnconfirmedBlocks(ctx context.Context) ([]*Block, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, height, hash, miner_id, worker_id, reward, difficulty, found_at, confirmed, orphaned
		FROM blocks
		WHERE confirmed = false AND orphaned = false
		ORDER BY height ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.ID, &b.Height, &b.Hash, &b.MinerID, &b.WorkerID,
			&b.Reward, &b.Difficulty, &b.FoundAt, &b.Confirmed, &b.Orphaned); err != nil {
			return nil, fmt.Errorf("failed to scan block: %w", err)
		}
		blocks = append(blocks, &b)
	}

	return blocks, nil
}

// ConfirmBlock marks a block as confirmed
func (db *DB) ConfirmBlock(ctx context.Context, blockID int64) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE blocks SET confirmed = true, confirmed_at = NOW() WHERE id = $1
	`, blockID)
	return err
}

// OrphanBlock marks a block as orphaned
func (db *DB) OrphanBlock(ctx context.Context, blockID int64) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE blocks SET orphaned = true WHERE id = $1
	`, blockID)
	return err
}

// SetWorkerOffline marks a worker as offline
func (db *DB) SetWorkerOffline(ctx context.Context, workerID int64) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE workers SET is_online = false WHERE id = $1
	`, workerID)
	return err
}

// GetPoolStats returns pool statistics
type PoolStats struct {
	TotalMiners    int64
	OnlineMiners   int64
	OnlineWorkers  int64
	TotalShares24h int64
	TotalBlocks    int64
	LastBlockTime  *time.Time
}

func (db *DB) GetPoolStats(ctx context.Context) (*PoolStats, error) {
	var stats PoolStats

	row := db.pool.QueryRow(ctx, `
		SELECT 
			(SELECT COUNT(*) FROM miners) as total_miners,
			(SELECT COUNT(DISTINCT miner_id) FROM workers WHERE is_online = true) as online_miners,
			(SELECT COUNT(*) FROM workers WHERE is_online = true) as online_workers,
			(SELECT COUNT(*) FROM shares WHERE timestamp > NOW() - INTERVAL '24 hours') as shares_24h,
			(SELECT COUNT(*) FROM blocks WHERE confirmed = true) as total_blocks,
			(SELECT MAX(found_at) FROM blocks) as last_block_time
	`)

	err := row.Scan(&stats.TotalMiners, &stats.OnlineMiners, &stats.OnlineWorkers,
		&stats.TotalShares24h, &stats.TotalBlocks, &stats.LastBlockTime)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return &stats, nil
}

// GetMinerByAddress retrieves a miner by wallet address
func (db *DB) GetMinerByAddress(ctx context.Context, address string) (*Miner, error) {
	var miner Miner
	err := db.pool.QueryRow(ctx, `
		SELECT id, address, email, created_at, last_seen_at, total_shares, total_blocks, pending_payout, total_paid
		FROM miners WHERE address = $1
	`, address).Scan(
		&miner.ID, &miner.Address, &miner.Email, &miner.CreatedAt, &miner.LastSeenAt,
		&miner.TotalShares, &miner.TotalBlocks, &miner.PendingPayout, &miner.TotalPaid,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get miner: %w", err)
	}

	return &miner, nil
}

// GetMinerWorkers retrieves all workers for a miner
func (db *DB) GetMinerWorkers(ctx context.Context, minerID int64) ([]*Worker, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, miner_id, name, agent, difficulty, created_at, last_seen_at, total_shares, is_online
		FROM workers WHERE miner_id = $1
		ORDER BY last_seen_at DESC
	`, minerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workers: %w", err)
	}
	defer rows.Close()

	var workers []*Worker
	for rows.Next() {
		var w Worker
		if err := rows.Scan(&w.ID, &w.MinerID, &w.Name, &w.Agent, &w.Difficulty,
			&w.CreatedAt, &w.LastSeenAt, &w.TotalShares, &w.IsOnline); err != nil {
			return nil, fmt.Errorf("failed to scan worker: %w", err)
		}
		workers = append(workers, &w)
	}

	return workers, nil
}

// BlockInfo holds block information for API responses
type BlockInfo struct {
	ID        int64     `json:"id"`
	Height    int64     `json:"height"`
	Hash      string    `json:"hash"`
	Reward    float64   `json:"reward"`
	MinerAddr string    `json:"miner"`
	FoundAt   time.Time `json:"found_at"`
	Confirmed bool      `json:"confirmed"`
	Orphaned  bool      `json:"orphaned"`
}

// QueryBlocks retrieves blocks with pagination
func (db *DB) QueryBlocks(ctx context.Context, limit, offset int) ([]*BlockInfo, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT b.id, b.height, b.hash, b.reward, m.address, b.found_at, b.confirmed, b.orphaned
		FROM blocks b
		LEFT JOIN miners m ON m.id = b.miner_id
		ORDER BY b.height DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*BlockInfo
	for rows.Next() {
		var b BlockInfo
		var addr *string
		if err := rows.Scan(&b.ID, &b.Height, &b.Hash, &b.Reward, &addr, &b.FoundAt, &b.Confirmed, &b.Orphaned); err != nil {
			return nil, fmt.Errorf("failed to scan block: %w", err)
		}
		if addr != nil {
			b.MinerAddr = *addr
		}
		blocks = append(blocks, &b)
	}

	return blocks, nil
}

// PaymentInfo holds payment information for API responses
type PaymentInfo struct {
	ID        int64     `json:"id"`
	MinerAddr string    `json:"address"`
	Amount    float64   `json:"amount"`
	TxID      string    `json:"txid,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// QueryPayments retrieves payments with pagination
func (db *DB) QueryPayments(ctx context.Context, limit, offset int) ([]*PaymentInfo, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT p.id, m.address, p.amount, p.txid, p.status, p.created_at
		FROM payouts p
		JOIN miners m ON m.id = p.miner_id
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []*PaymentInfo
	for rows.Next() {
		var p PaymentInfo
		var txid *string
		if err := rows.Scan(&p.ID, &p.MinerAddr, &p.Amount, &txid, &p.Status, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		if txid != nil {
			p.TxID = *txid
		}
		payments = append(payments, &p)
	}

	return payments, nil
}

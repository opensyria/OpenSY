// Package payout implements PPLNS (Pay Per Last N Shares) reward distribution.
// PPLNS is fairer than PPS because it discourages pool hopping.
package payout

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds payout configuration
type Config struct {
	WindowSize     int64         // PPLNS window size (number of shares)
	MinPayout      int64         // Minimum payout threshold in satoshis
	PoolFeePercent float64       // Pool fee percentage (e.g., 1.0 = 1%)
	PayoutInterval time.Duration // Payout interval
	PoolFeeAddress string        // Pool wallet address for fees
	Logger         *slog.Logger
}

// DefaultConfig returns default payout configuration
func DefaultConfig() Config {
	return Config{
		WindowSize:     100000,
		MinPayout:      100000000, // 1 SYL minimum
		PoolFeePercent: 1.0,       // 1% fee
		PayoutInterval: 1 * time.Hour,
		Logger:         slog.Default(),
	}
}

// PPLNS implements the Pay Per Last N Shares payout scheme
type PPLNS struct {
	cfg    Config
	db     *pgxpool.Pool
	logger *slog.Logger
}

// New creates a new PPLNS payout calculator
func New(cfg Config, db *pgxpool.Pool) *PPLNS {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &PPLNS{
		cfg:    cfg,
		db:     db,
		logger: cfg.Logger.With("component", "pplns"),
	}
}

// ShareWindow represents a miner's contribution in the PPLNS window
type ShareWindow struct {
	MinerID     int64
	Address     string
	TotalShares int64
	TotalDiff   int64
	Percentage  float64
	Reward      int64
}

// BlockPayout represents a pending block payout
type BlockPayout struct {
	BlockID      int64
	BlockHeight  int64
	BlockHash    string
	TotalReward  int64
	PoolFee      int64
	MinerRewards []MinerReward
	CalculatedAt time.Time
}

// MinerReward represents a single miner's reward from a block
type MinerReward struct {
	MinerID    int64
	Address    string
	Shares     int64
	Difficulty int64
	Percentage float64
	Amount     int64
}

// CalculateBlockPayout calculates rewards for a confirmed block using PPLNS
func (p *PPLNS) CalculateBlockPayout(ctx context.Context, blockID int64) (*BlockPayout, error) {
	var block struct {
		Height  int64
		Hash    string
		Reward  int64
		FoundAt time.Time
	}

	err := p.db.QueryRow(ctx, `
		SELECT height, hash, reward, found_at
		FROM blocks
		WHERE id = $1 AND confirmed = true AND orphaned = false
	`, blockID).Scan(&block.Height, &block.Hash, &block.Reward, &block.FoundAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("block %d not found or not confirmed", blockID)
		}
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	shares, err := p.getShareWindow(ctx, block.FoundAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get share window: %w", err)
	}

	if len(shares) == 0 {
		return nil, fmt.Errorf("no shares in window for block %d", blockID)
	}

	var totalDiff int64
	for _, s := range shares {
		totalDiff += s.TotalDiff
	}

	poolFee := int64(float64(block.Reward) * (p.cfg.PoolFeePercent / 100.0))
	distributableReward := block.Reward - poolFee

	rewards := make([]MinerReward, 0, len(shares))
	var distributedTotal int64

	for i, share := range shares {
		reward := new(big.Int).SetInt64(distributableReward)
		reward.Mul(reward, big.NewInt(share.TotalDiff))
		reward.Div(reward, big.NewInt(totalDiff))

		minerReward := reward.Int64()
		distributedTotal += minerReward

		percentage := float64(share.TotalDiff) / float64(totalDiff) * 100.0

		rewards = append(rewards, MinerReward{
			MinerID:    share.MinerID,
			Address:    share.Address,
			Shares:     share.TotalShares,
			Difficulty: share.TotalDiff,
			Percentage: percentage,
			Amount:     minerReward,
		})

		shares[i].Percentage = percentage
		shares[i].Reward = minerReward
	}

	dust := distributableReward - distributedTotal
	if dust > 0 && len(rewards) > 0 {
		rewards[0].Amount += dust
	}

	payout := &BlockPayout{
		BlockID:      blockID,
		BlockHeight:  block.Height,
		BlockHash:    block.Hash,
		TotalReward:  block.Reward,
		PoolFee:      poolFee,
		MinerRewards: rewards,
		CalculatedAt: time.Now(),
	}

	p.logger.Info("Block payout calculated",
		"block", blockID,
		"height", block.Height,
		"reward", block.Reward,
		"fee", poolFee,
		"miners", len(rewards),
	)

	return payout, nil
}

func (p *PPLNS) getShareWindow(ctx context.Context, beforeTime time.Time) ([]ShareWindow, error) {
	rows, err := p.db.Query(ctx, `
		WITH window_shares AS (
			SELECT 
				miner_id,
				difficulty,
				ROW_NUMBER() OVER (ORDER BY timestamp DESC) as rn
			FROM shares
			WHERE timestamp <= $1 AND is_valid = true
		)
		SELECT 
			ws.miner_id,
			m.address,
			COUNT(*) as share_count,
			SUM(ws.difficulty) as total_diff
		FROM window_shares ws
		JOIN miners m ON m.id = ws.miner_id
		WHERE ws.rn <= $2
		GROUP BY ws.miner_id, m.address
		ORDER BY total_diff DESC
	`, beforeTime, p.cfg.WindowSize)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []ShareWindow
	for rows.Next() {
		var s ShareWindow
		if err := rows.Scan(&s.MinerID, &s.Address, &s.TotalShares, &s.TotalDiff); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}

	return shares, rows.Err()
}

// SaveBlockPayout saves the calculated payout to the database
func (p *PPLNS) SaveBlockPayout(ctx context.Context, payout *BlockPayout) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var payoutID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO payouts (block_id, total_reward, pool_fee, calculated_at, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id
	`, payout.BlockID, payout.TotalReward, payout.PoolFee, payout.CalculatedAt).Scan(&payoutID)

	if err != nil {
		return fmt.Errorf("failed to insert payout: %w", err)
	}

	for _, reward := range payout.MinerRewards {
		_, err = tx.Exec(ctx, `
			INSERT INTO payout_details (payout_id, miner_id, shares, difficulty, percentage, amount)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, payoutID, reward.MinerID, reward.Shares, reward.Difficulty, reward.Percentage, reward.Amount)

		if err != nil {
			return fmt.Errorf("failed to insert payout detail: %w", err)
		}

		_, err = tx.Exec(ctx, `
			UPDATE miners SET pending_payout = pending_payout + $1 WHERE id = $2
		`, reward.Amount, reward.MinerID)

		if err != nil {
			return fmt.Errorf("failed to update miner balance: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE blocks SET payout_id = $1 WHERE id = $2
	`, payoutID, payout.BlockID)

	if err != nil {
		return fmt.Errorf("failed to update block: %w", err)
	}

	return tx.Commit(ctx)
}

// PendingPayout represents a miner's pending withdrawal
type PendingPayout struct {
	MinerID int64
	Address string
	Amount  int64
}

// GetPendingPayouts returns miners with balances above minimum threshold
func (p *PPLNS) GetPendingPayouts(ctx context.Context) ([]PendingPayout, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, address, pending_payout
		FROM miners
		WHERE pending_payout >= $1
		ORDER BY pending_payout DESC
	`, p.cfg.MinPayout)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payouts []PendingPayout
	for rows.Next() {
		var pp PendingPayout
		if err := rows.Scan(&pp.MinerID, &pp.Address, &pp.Amount); err != nil {
			return nil, err
		}
		payouts = append(payouts, pp)
	}

	return payouts, rows.Err()
}

// MinerPayoutStats holds payout statistics for a miner
type MinerPayoutStats struct {
	PendingBalance int64
	TotalPaid      int64
	TotalEarned    int64
	PayoutCount    int
	RecentPayouts  []RecentPayout
}

// RecentPayout represents a recent payout entry
type RecentPayout struct {
	Amount      int64
	Percentage  float64
	Timestamp   time.Time
	BlockHeight int64
}

// GetMinerStats returns payout statistics for a miner
func (p *PPLNS) GetMinerStats(ctx context.Context, minerID int64) (*MinerPayoutStats, error) {
	var stats MinerPayoutStats

	err := p.db.QueryRow(ctx, `
		SELECT 
			pending_payout,
			total_paid,
			(SELECT COUNT(*) FROM payout_details WHERE miner_id = $1) as payout_count,
			(SELECT COALESCE(SUM(amount), 0) FROM payout_details WHERE miner_id = $1) as total_earned
		FROM miners
		WHERE id = $1
	`, minerID).Scan(&stats.PendingBalance, &stats.TotalPaid, &stats.PayoutCount, &stats.TotalEarned)

	if err != nil {
		return nil, err
	}

	rows, err := p.db.Query(ctx, `
		SELECT pd.amount, pd.percentage, p.calculated_at, b.height
		FROM payout_details pd
		JOIN payouts p ON p.id = pd.payout_id
		JOIN blocks b ON b.id = p.block_id
		WHERE pd.miner_id = $1
		ORDER BY p.calculated_at DESC
		LIMIT 10
	`, minerID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rp RecentPayout
		if err := rows.Scan(&rp.Amount, &rp.Percentage, &rp.Timestamp, &rp.BlockHeight); err != nil {
			return nil, err
		}
		stats.RecentPayouts = append(stats.RecentPayouts, rp)
	}

	return &stats, nil
}

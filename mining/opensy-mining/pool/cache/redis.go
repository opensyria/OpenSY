// Package cache provides Redis caching for the mining pool.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Config holds Redis configuration
type Config struct {
	Addr     string
	Password string
	DB       int
}

// Cache wraps the Redis client
type Cache struct {
	client *redis.Client
}

// New creates a new Redis cache
func New(cfg Config) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Cache{client: client}, nil
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	return c.client.Close()
}

// Session caching

// SessionData holds cached session information
type SessionData struct {
	ID         string `json:"id"`
	MinerID    int64  `json:"miner_id"`
	WorkerID   int64  `json:"worker_id"`
	Login      string `json:"login"`
	WorkerName string `json:"worker_name"`
	Difficulty uint64 `json:"difficulty"`
}

// SetSession caches session data
func (c *Cache) SetSession(ctx context.Context, sessionID string, data *SessionData, ttl time.Duration) error {
	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	return c.client.Set(ctx, "session:"+sessionID, value, ttl).Err()
}

// GetSession retrieves cached session data
func (c *Cache) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	value, err := c.client.Get(ctx, "session:"+sessionID).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(value, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &data, nil
}

// DeleteSession removes a cached session
func (c *Cache) DeleteSession(ctx context.Context, sessionID string) error {
	return c.client.Del(ctx, "session:"+sessionID).Err()
}

// Share duplicate detection

// CheckShareDuplicate checks if a share was already submitted (returns true if duplicate)
func (c *Cache) CheckShareDuplicate(ctx context.Context, sessionID, jobID, nonce string) (bool, error) {
	key := fmt.Sprintf("share:%s:%s:%s", sessionID, jobID, nonce)

	// SETNX returns false if key already exists
	set, err := c.client.SetNX(ctx, key, "1", 5*time.Minute).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check share: %w", err)
	}

	// If set is false, key already existed (duplicate)
	return !set, nil
}

// Job caching

// SetCurrentJob caches the current job blob
func (c *Cache) SetCurrentJob(ctx context.Context, height int64, blob string) error {
	return c.client.Set(ctx, "job:current", fmt.Sprintf("%d:%s", height, blob), 5*time.Minute).Err()
}

// Block template caching

// SetBlockTemplate caches the current block template
func (c *Cache) SetBlockTemplate(ctx context.Context, template []byte) error {
	return c.client.Set(ctx, "template:current", template, time.Minute).Err()
}

// GetBlockTemplate retrieves the cached block template
func (c *Cache) GetBlockTemplate(ctx context.Context) ([]byte, error) {
	value, err := c.client.Get(ctx, "template:current").Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return value, err
}

// Hashrate calculation

// RecordShare records a share for hashrate calculation
func (c *Cache) RecordShare(ctx context.Context, minerID int64, workerID int64, difficulty uint64) error {
	now := time.Now()
	bucket := now.Unix() / 60 // 1-minute buckets

	pipe := c.client.Pipeline()

	// Pool hashrate
	poolKey := fmt.Sprintf("hashrate:pool:%d", bucket)
	pipe.IncrBy(ctx, poolKey, int64(difficulty))
	pipe.Expire(ctx, poolKey, 10*time.Minute)

	// Miner hashrate
	minerKey := fmt.Sprintf("hashrate:miner:%d:%d", minerID, bucket)
	pipe.IncrBy(ctx, minerKey, int64(difficulty))
	pipe.Expire(ctx, minerKey, 10*time.Minute)

	// Worker hashrate
	workerKey := fmt.Sprintf("hashrate:worker:%d:%d", workerID, bucket)
	pipe.IncrBy(ctx, workerKey, int64(difficulty))
	pipe.Expire(ctx, workerKey, 10*time.Minute)

	_, err := pipe.Exec(ctx)
	return err
}

// GetPoolHashrate calculates pool hashrate over the last N minutes
func (c *Cache) GetPoolHashrate(ctx context.Context, minutes int) (float64, error) {
	now := time.Now().Unix() / 60

	var totalDiff int64
	for i := 0; i < minutes; i++ {
		bucket := now - int64(i)
		key := fmt.Sprintf("hashrate:pool:%d", bucket)
		val, err := c.client.Get(ctx, key).Int64()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return 0, err
		}
		totalDiff += val
	}

	// Convert to H/s: totalDiff / (minutes * 60)
	return float64(totalDiff) / float64(minutes*60), nil
}

// GetMinerHashrate calculates miner hashrate over the last N minutes
func (c *Cache) GetMinerHashrate(ctx context.Context, minerID int64, minutes int) (float64, error) {
	now := time.Now().Unix() / 60

	var totalDiff int64
	for i := 0; i < minutes; i++ {
		bucket := now - int64(i)
		key := fmt.Sprintf("hashrate:miner:%d:%d", minerID, bucket)
		val, err := c.client.Get(ctx, key).Int64()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return 0, err
		}
		totalDiff += val
	}

	return float64(totalDiff) / float64(minutes*60), nil
}

// Pub/Sub for job notifications

// PublishNewJob publishes a new job notification
func (c *Cache) PublishNewJob(ctx context.Context, height int64) error {
	return c.client.Publish(ctx, "jobs:new", height).Err()
}

// SubscribeJobs subscribes to new job notifications
func (c *Cache) SubscribeJobs(ctx context.Context) *redis.PubSub {
	return c.client.Subscribe(ctx, "jobs:new")
}

// Online worker tracking

// SetWorkerOnline marks a worker as online
func (c *Cache) SetWorkerOnline(ctx context.Context, workerID int64) error {
	return c.client.SAdd(ctx, "workers:online", workerID).Err()
}

// SetWorkerOffline removes a worker from online set
func (c *Cache) SetWorkerOffline(ctx context.Context, workerID int64) error {
	return c.client.SRem(ctx, "workers:online", workerID).Err()
}

// GetOnlineWorkerCount returns the number of online workers
func (c *Cache) GetOnlineWorkerCount(ctx context.Context) (int64, error) {
	return c.client.SCard(ctx, "workers:online").Result()
}

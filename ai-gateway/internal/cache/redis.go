// Package cache provides Redis-backed distributed rate limiting and caching.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis client with rate-limiting primitives.
type Client struct {
	rdb *redis.Client
}

// Config holds Redis connection parameters.
type Config struct {
	Addr     string `yaml:"addr"`         // e.g. "localhost:6379"
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`            // Redis database number
}

// New creates a new Redis client. Returns nil if addr is empty (Redis disabled).
func New(cfg Config) *Client {
	if cfg.Addr == "" {
		return nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb: rdb}
}

// Close shuts down the Redis connection.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// Ping checks connectivity. Returns nil if reachable.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis not configured")
	}
	return c.rdb.Ping(ctx).Err()
}

// Allow checks whether a rate-limited key is allowed.
// Uses the sliding window approach: a sorted set tracks timestamps per key.
// Returns (allowed, remaining, error).
func (c *Client) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	if c == nil || c.rdb == nil {
		return true, limit, nil // Redis not configured, allow all
	}

	now := float64(time.Now().UnixNano()) / 1e9
	windowStart := now - window.Seconds()

	pipe := c.rdb.Pipeline()

	// Remove entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%f", windowStart))

	// Count remaining entries
	countCmd := pipe.ZCard(ctx, key)

	// Add current request timestamp
	pipe.ZAdd(ctx, key, redis.Z{Score: now, Member: now})

	// Set TTL on the key to auto-clean
	pipe.Expire(ctx, key, window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}

	count, err := countCmd.Result()
	if err != nil {
		return false, 0, err
	}

	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return int(count) <= limit, remaining, nil
}

// Reset clears all rate limit data for a key.
func (c *Client) Reset(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, key).Err()
}

// SetRaw stores arbitrary byte data under a key.
func (c *Client) SetRaw(ctx context.Context, key string, data []byte) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, key, data, 0).Err()
}

// GetRaw retrieves arbitrary byte data for a key. Returns (nil, nil) if not found.
func (c *Client) GetRaw(ctx context.Context, key string) ([]byte, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}


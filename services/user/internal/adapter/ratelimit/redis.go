// Package ratelimit provides rate limiting functionality using Redis.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter implements rate limiting using Redis.
type RedisRateLimiter struct {
	client     *redis.Client
	maxAttempts int
	window      time.Duration
	keyPrefix   string
}

// Config holds rate limiter configuration.
type Config struct {
	MaxAttempts int           // Maximum attempts allowed within window
	Window      time.Duration // Time window for rate limiting
	KeyPrefix   string        // Prefix for Redis keys
}

// DefaultConfig returns default rate limiter configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 5,
		Window:      15 * time.Minute,
		KeyPrefix:   "ratelimit:login:",
	}
}

// NewRedisRateLimiter creates a new Redis-based rate limiter.
func NewRedisRateLimiter(client *redis.Client, cfg Config) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:      client,
		maxAttempts: cfg.MaxAttempts,
		window:      cfg.Window,
		keyPrefix:   cfg.KeyPrefix,
	}
}

// Allow checks if an attempt is allowed for the given key.
// Returns true if allowed, false if rate limited.
func (r *RedisRateLimiter) Allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Hash the key for privacy (don't store plain email in Redis)
	hashedKey := r.hashKey(key)
	redisKey := r.keyPrefix + hashedKey

	// Use INCR to atomically increment the counter
	count, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		// On error, allow the request (fail open for availability)
		return true
	}

	// Set expiration on first increment
	if count == 1 {
		r.client.Expire(ctx, redisKey, r.window)
	}

	return count <= int64(r.maxAttempts)
}

// Reset clears the rate limit counter for the given key.
func (r *RedisRateLimiter) Reset(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hashedKey := r.hashKey(key)
	redisKey := r.keyPrefix + hashedKey

	return r.client.Del(ctx, redisKey).Err()
}

// GetRemainingAttempts returns the number of remaining attempts for a key.
func (r *RedisRateLimiter) GetRemainingAttempts(key string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hashedKey := r.hashKey(key)
	redisKey := r.keyPrefix + hashedKey

	count, err := r.client.Get(ctx, redisKey).Int()
	if err == redis.Nil {
		return r.maxAttempts, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get rate limit counter: %w", err)
	}

	remaining := r.maxAttempts - count
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// GetTimeUntilReset returns the time until the rate limit resets for a key.
func (r *RedisRateLimiter) GetTimeUntilReset(key string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hashedKey := r.hashKey(key)
	redisKey := r.keyPrefix + hashedKey

	ttl, err := r.client.TTL(ctx, redisKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}

	if ttl < 0 {
		return 0, nil
	}

	return ttl, nil
}

// hashKey creates a SHA-256 hash of the key for privacy.
func (r *RedisRateLimiter) hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

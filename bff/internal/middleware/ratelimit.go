package middleware

import (
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	FailureThreshold int
	Window           time.Duration
	Cooldown         time.Duration
}

// ipState tracks failures for a single IP.
type ipState struct {
	failureCount  int
	windowStart   time.Time
	cooldownUntil time.Time
}

// RateLimiter implements IP-based rate limiting for auth failures.
type RateLimiter struct {
	config RateLimitConfig
	state  map[string]*ipState
	mu     sync.RWMutex
	done   chan struct{}
}

// NewRateLimiter creates a new rate limiter with background cleanup.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	r := &RateLimiter{
		config: config,
		state:  make(map[string]*ipState),
		done:   make(chan struct{}),
	}
	go r.cleanup()
	return r
}

// cleanup periodically removes expired IP entries to prevent memory leaks.
func (r *RateLimiter) cleanup() {
	ticker := time.NewTicker(r.config.Window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for ip, state := range r.state {
				expired := now.Sub(state.windowStart) > r.config.Window*2
				cooledDown := state.cooldownUntil.IsZero() || now.After(state.cooldownUntil)
				if expired && cooledDown {
					delete(r.state, ip)
				}
			}
			r.mu.Unlock()
		case <-r.done:
			return
		}
	}
}

// Close stops the background cleanup goroutine.
func (r *RateLimiter) Close() {
	close(r.done)
}

// IsRateLimited checks if an IP is currently rate limited.
func (r *RateLimiter) IsRateLimited(ip string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.state[ip]
	if !exists {
		return false
	}

	// Check if in cooldown
	if !state.cooldownUntil.IsZero() && time.Now().Before(state.cooldownUntil) {
		return true
	}

	return false
}

// RecordFailure records an authentication failure for an IP.
func (r *RateLimiter) RecordFailure(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	state, exists := r.state[ip]
	if !exists {
		state = &ipState{
			windowStart: now,
		}
		r.state[ip] = state
	}

	// Check if cooldown has expired
	if !state.cooldownUntil.IsZero() && now.After(state.cooldownUntil) {
		// Reset state after cooldown
		state.failureCount = 0
		state.windowStart = now
		state.cooldownUntil = time.Time{}
	}

	// Check if window has expired
	if now.Sub(state.windowStart) > r.config.Window {
		state.failureCount = 0
		state.windowStart = now
	}

	state.failureCount++

	// Check if threshold reached
	if state.failureCount >= r.config.FailureThreshold {
		state.cooldownUntil = now.Add(r.config.Cooldown)
		return true
	}

	return false
}

// Reset clears rate limit state for an IP.
func (r *RateLimiter) Reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.state, ip)
}

// GetFailureCount returns the current failure count for an IP.
func (r *RateLimiter) GetFailureCount(ip string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.state[ip]
	if !exists {
		return 0
	}

	return state.failureCount
}

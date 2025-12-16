package middleware_test

import (
	"sync"
	"testing"
	"time"

	"github.com/daisuke8000/example-ec-platform/bff/internal/middleware"
)

func TestRateLimiter_AllowsUnderThreshold(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 5,
		Window:           time.Minute,
		Cooldown:         5 * time.Minute,
	}

	rl := middleware.NewRateLimiter(cfg)

	ip := "192.168.1.1"

	// Should allow requests under threshold
	for i := 0; i < 4; i++ {
		if rl.IsRateLimited(ip) {
			t.Errorf("expected IP to not be rate limited after %d failures", i)
		}
		rl.RecordFailure(ip)
	}
}

func TestRateLimiter_BlocksAtThreshold(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 3,
		Window:           time.Minute,
		Cooldown:         5 * time.Minute,
	}

	rl := middleware.NewRateLimiter(cfg)

	ip := "192.168.1.2"

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		rl.RecordFailure(ip)
	}

	// Should now be rate limited
	if !rl.IsRateLimited(ip) {
		t.Error("expected IP to be rate limited after reaching threshold")
	}
}

func TestRateLimiter_CooldownExpiry(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		Cooldown:         100 * time.Millisecond, // Short for testing
	}

	rl := middleware.NewRateLimiter(cfg)

	ip := "192.168.1.3"

	// Trigger rate limit
	rl.RecordFailure(ip)
	rl.RecordFailure(ip)

	if !rl.IsRateLimited(ip) {
		t.Error("expected IP to be rate limited")
	}

	// Wait for cooldown
	time.Sleep(150 * time.Millisecond)

	// Should no longer be rate limited
	if rl.IsRateLimited(ip) {
		t.Error("expected IP to not be rate limited after cooldown")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 3,
		Window:           100 * time.Millisecond, // Short for testing
		Cooldown:         5 * time.Minute,
	}

	rl := middleware.NewRateLimiter(cfg)

	ip := "192.168.1.4"

	// Record 2 failures
	rl.RecordFailure(ip)
	rl.RecordFailure(ip)

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Record 1 more failure - should reset window
	rl.RecordFailure(ip)

	// Should not be rate limited (only 1 failure in new window)
	if rl.IsRateLimited(ip) {
		t.Error("expected IP to not be rate limited after window expiry")
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 100,
		Window:           time.Minute,
		Cooldown:         5 * time.Minute,
	}

	rl := middleware.NewRateLimiter(cfg)

	ip := "192.168.1.5"

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.RecordFailure(ip)
			rl.IsRateLimited(ip)
		}()
	}

	wg.Wait()

	// Should not panic or deadlock
	count := rl.GetFailureCount(ip)
	if count != 50 {
		t.Errorf("expected 50 failures, got %d", count)
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		Cooldown:         5 * time.Minute,
	}

	rl := middleware.NewRateLimiter(cfg)

	ip1 := "192.168.1.10"
	ip2 := "192.168.1.20"

	// Rate limit ip1
	rl.RecordFailure(ip1)
	rl.RecordFailure(ip1)

	// ip1 should be rate limited
	if !rl.IsRateLimited(ip1) {
		t.Error("expected ip1 to be rate limited")
	}

	// ip2 should not be rate limited
	if rl.IsRateLimited(ip2) {
		t.Error("expected ip2 to not be rate limited")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	cfg := middleware.RateLimitConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		Cooldown:         5 * time.Minute,
	}

	rl := middleware.NewRateLimiter(cfg)

	ip := "192.168.1.6"

	// Trigger rate limit
	rl.RecordFailure(ip)
	rl.RecordFailure(ip)

	if !rl.IsRateLimited(ip) {
		t.Error("expected IP to be rate limited")
	}

	// Reset
	rl.Reset(ip)

	// Should no longer be rate limited
	if rl.IsRateLimited(ip) {
		t.Error("expected IP to not be rate limited after reset")
	}
}

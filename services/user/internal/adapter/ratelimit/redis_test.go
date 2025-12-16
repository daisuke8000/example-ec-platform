package ratelimit

import (
	"testing"
)

func TestHashKey(t *testing.T) {
	cfg := DefaultConfig()
	rl := &RedisRateLimiter{
		maxAttempts: cfg.MaxAttempts,
		window:      cfg.Window,
		keyPrefix:   cfg.KeyPrefix,
	}

	// Same input should produce same hash
	hash1 := rl.hashKey("test@example.com")
	hash2 := rl.hashKey("test@example.com")
	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}

	// Different input should produce different hash
	hash3 := rl.hashKey("other@example.com")
	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}

	// Hash should be 64 characters (256 bits = 32 bytes = 64 hex chars)
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}

	if cfg.Window.Minutes() != 15 {
		t.Errorf("Window = %v, want 15 minutes", cfg.Window)
	}

	if cfg.KeyPrefix != "ratelimit:login:" {
		t.Errorf("KeyPrefix = %q, want %q", cfg.KeyPrefix, "ratelimit:login:")
	}
}

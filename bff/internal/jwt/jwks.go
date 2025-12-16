package jwt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JWKSConfig holds configuration for JWKS management.
type JWKSConfig struct {
	URL                string
	RefreshInterval    time.Duration
	MinRefreshInterval time.Duration
}

// JWKSManager manages JWKS fetching and caching.
type JWKSManager struct {
	cache              *jwk.Cache
	url                string
	minRefreshInterval time.Duration
	lastRefresh        time.Time
	refreshMu          sync.Mutex
	healthy            bool
	healthMu           sync.RWMutex
}

// KeyNotFoundError indicates the requested key ID was not found in JWKS.
type KeyNotFoundError struct {
	KID string
}

func (e *KeyNotFoundError) Error() string {
	return fmt.Sprintf("key not found: %s", e.KID)
}

// IsKeyNotFoundError checks if the error is a KeyNotFoundError.
func IsKeyNotFoundError(err error) bool {
	var knf *KeyNotFoundError
	return errors.As(err, &knf)
}

// NewJWKSManager creates a new JWKS manager with initial fetch.
// Returns error if initial JWKS fetch fails.
func NewJWKSManager(ctx context.Context, cfg JWKSConfig) (*JWKSManager, error) {
	if cfg.MinRefreshInterval < 10*time.Second {
		cfg.MinRefreshInterval = 10 * time.Second
	}

	cache := jwk.NewCache(ctx)

	err := cache.Register(cfg.URL,
		jwk.WithMinRefreshInterval(cfg.MinRefreshInterval),
		jwk.WithRefreshInterval(cfg.RefreshInterval),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register JWKS URL: %w", err)
	}

	// Initial fetch to ensure JWKS is available
	_, err = cache.Refresh(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch initial JWKS: %w", err)
	}

	m := &JWKSManager{
		cache:              cache,
		url:                cfg.URL,
		minRefreshInterval: cfg.MinRefreshInterval,
		lastRefresh:        time.Now(),
		healthy:            true,
	}

	return m, nil
}

// GetKey retrieves a public key by its Key ID.
func (m *JWKSManager) GetKey(ctx context.Context, kid string) (jwk.Key, error) {
	set, err := m.cache.Get(ctx, m.url)
	if err != nil {
		m.setHealthy(false)
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	key, found := set.LookupKeyID(kid)
	if !found {
		// Try refresh once for unknown kid
		if err := m.Refresh(ctx); err == nil {
			set, err = m.cache.Get(ctx, m.url)
			if err == nil {
				key, found = set.LookupKeyID(kid)
			}
		}
	}

	if !found {
		return nil, &KeyNotFoundError{KID: kid}
	}

	m.setHealthy(true)
	return key, nil
}

// Refresh forces a JWKS refresh, respecting MinRefreshInterval.
func (m *JWKSManager) Refresh(ctx context.Context) error {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()

	if time.Since(m.lastRefresh) < m.minRefreshInterval {
		return nil // Throttled
	}

	_, err := m.cache.Refresh(ctx, m.url)
	if err != nil {
		m.setHealthy(false)
		return fmt.Errorf("failed to refresh JWKS: %w", err)
	}

	m.lastRefresh = time.Now()
	m.setHealthy(true)
	return nil
}

// IsHealthy returns true if the last JWKS operation was successful.
func (m *JWKSManager) IsHealthy() bool {
	m.healthMu.RLock()
	defer m.healthMu.RUnlock()
	return m.healthy
}

// GetKeyCount returns the number of keys in the cached JWKS.
func (m *JWKSManager) GetKeyCount() int {
	set, err := m.cache.Get(context.Background(), m.url)
	if err != nil {
		return 0
	}
	return set.Len()
}

// Close marks the manager as unhealthy for graceful shutdown.
func (m *JWKSManager) Close() {
	m.setHealthy(false)
}

func (m *JWKSManager) setHealthy(healthy bool) {
	m.healthMu.Lock()
	defer m.healthMu.Unlock()
	m.healthy = healthy
}

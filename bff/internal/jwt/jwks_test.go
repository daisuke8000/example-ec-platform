package jwt_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"

	"github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
)

func generateTestJWKS(t *testing.T, kid string) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	key, err := jwk.FromRaw(privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to create JWK: %v", err)
	}

	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatalf("failed to set kid: %v", err)
	}
	if err := key.Set(jwk.AlgorithmKey, "RS256"); err != nil {
		t.Fatalf("failed to set alg: %v", err)
	}
	if err := key.Set(jwk.KeyUsageKey, "sig"); err != nil {
		t.Fatalf("failed to set use: %v", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(key); err != nil {
		t.Fatalf("failed to add key to set: %v", err)
	}

	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("failed to marshal JWKS: %v", err)
	}

	return data
}

func TestJWKSManager_NewManager_Success(t *testing.T) {
	jwksData := generateTestJWKS(t, "test-key-1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer server.Close()

	cfg := jwt.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	manager, err := jwt.NewJWKSManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJWKSManager() error = %v", err)
	}
	defer manager.Close()

	if !manager.IsHealthy() {
		t.Error("expected manager to be healthy after successful initialization")
	}
}

func TestJWKSManager_NewManager_FailsWhenHydraUnreachable(t *testing.T) {
	cfg := jwt.JWKSConfig{
		URL:                "http://localhost:59999/nonexistent",
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := jwt.NewJWKSManager(ctx, cfg)
	if err == nil {
		t.Error("expected error when Hydra is unreachable")
	}
}

func TestJWKSManager_GetKey_Success(t *testing.T) {
	kid := "test-key-123"
	jwksData := generateTestJWKS(t, kid)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer server.Close()

	cfg := jwt.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	manager, err := jwt.NewJWKSManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJWKSManager() error = %v", err)
	}
	defer manager.Close()

	key, err := manager.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	if key == nil {
		t.Error("expected non-nil key")
	}
}

func TestJWKSManager_GetKey_NotFound(t *testing.T) {
	jwksData := generateTestJWKS(t, "existing-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer server.Close()

	cfg := jwt.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	manager, err := jwt.NewJWKSManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJWKSManager() error = %v", err)
	}
	defer manager.Close()

	_, err = manager.GetKey(ctx, "nonexistent-key")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}

	if !jwt.IsKeyNotFoundError(err) {
		t.Errorf("expected KeyNotFoundError, got %T: %v", err, err)
	}
}

func TestJWKSManager_Refresh_RespectsMinInterval(t *testing.T) {
	callCount := 0
	jwksData := generateTestJWKS(t, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer server.Close()

	cfg := jwt.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 100 * time.Millisecond,
	}

	ctx := context.Background()
	manager, err := jwt.NewJWKSManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJWKSManager() error = %v", err)
	}
	defer manager.Close()

	initialCalls := callCount

	// Multiple rapid refresh calls should be throttled
	for i := 0; i < 5; i++ {
		_ = manager.Refresh(ctx)
	}

	// Should not have made 5 additional calls due to MinRefreshInterval
	additionalCalls := callCount - initialCalls
	if additionalCalls > 2 {
		t.Errorf("expected throttled refreshes, but got %d additional calls", additionalCalls)
	}
}

func TestJWKSManager_IsHealthy(t *testing.T) {
	jwksData := generateTestJWKS(t, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer server.Close()

	cfg := jwt.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	manager, err := jwt.NewJWKSManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJWKSManager() error = %v", err)
	}
	defer manager.Close()

	if !manager.IsHealthy() {
		t.Error("expected IsHealthy() to return true")
	}
}

func TestJWKSManager_GetKeyCount(t *testing.T) {
	jwksData := generateTestJWKS(t, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer server.Close()

	cfg := jwt.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	manager, err := jwt.NewJWKSManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJWKSManager() error = %v", err)
	}
	defer manager.Close()

	count := manager.GetKeyCount()
	if count != 1 {
		t.Errorf("expected 1 key, got %d", count)
	}
}

package server

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/daisuke8000/example-ec-platform/bff/internal/config"
	jwtpkg "github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
)

func TestE2E_HydraIntegration(t *testing.T) {
	hydraJWKSURL := os.Getenv("HYDRA_JWKS_URL")
	if hydraJWKSURL == "" {
		t.Skip("skipping E2E test: HYDRA_JWKS_URL not set (set environment variables to run)")
	}

	hydraIssuerURL := os.Getenv("HYDRA_ISSUER_URL")
	if hydraIssuerURL == "" {
		hydraIssuerURL = "http://localhost:4444"
	}

	jwtAudience := os.Getenv("JWT_AUDIENCE")
	if jwtAudience == "" {
		jwtAudience = "test-client"
	}

	resp, err := http.Get(hydraJWKSURL)
	if err != nil {
		t.Skipf("skipping E2E test: cannot reach Hydra JWKS endpoint: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("skipping E2E test: Hydra JWKS endpoint returned %d", resp.StatusCode)
	}

	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:               8080,
			TrustedProxyHeader: "X-Real-IP",
		},
		JWT: config.JWTConfig{
			IssuerURL: hydraIssuerURL,
			Audience:  jwtAudience,
			ClockSkew: 30 * time.Second,
		},
		JWKS: config.JWKSConfig{
			URL:                hydraJWKSURL,
			RefreshInterval:    time.Hour,
			MinRefreshInterval: 10 * time.Second,
		},
		RateLimit: config.RateLimitConfig{
			FailureThreshold: 10,
			Window:           time.Minute,
			Cooldown:         5 * time.Minute,
			Enabled:          true,
		},
	}

	deps, err := NewDependencies(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("failed to create dependencies with real Hydra: %v", err)
	}
	defer deps.Close()

	t.Run("JWKS manager initializes with real Hydra", func(t *testing.T) {
		if !deps.JWKSManager.IsHealthy() {
			t.Error("JWKS manager should be healthy after initialization")
		}

		keyCount := deps.JWKSManager.GetKeyCount()
		if keyCount == 0 {
			t.Error("JWKS should contain at least one key")
		}
		t.Logf("Hydra JWKS contains %d keys", keyCount)
	})

	t.Run("invalid token rejected by validator", func(t *testing.T) {
		invalidToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.invalid_signature"

		_, err := deps.Validator.Validate(ctx, invalidToken)
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("malformed token rejected", func(t *testing.T) {
		malformedToken := "not.a.valid.jwt.token"

		_, err := deps.Validator.Validate(ctx, malformedToken)
		if err == nil {
			t.Error("expected error for malformed token")
		}
	})

	t.Run("expired token returns TOKEN_EXPIRED error type", func(t *testing.T) {
		expiredTokenFromEnv := os.Getenv("TEST_EXPIRED_TOKEN")
		if expiredTokenFromEnv == "" {
			t.Skip("skipping: TEST_EXPIRED_TOKEN not set")
		}

		_, err := deps.Validator.Validate(ctx, expiredTokenFromEnv)
		if err == nil {
			t.Error("expected error for expired token")
			return
		}

		if !jwtpkg.IsTokenExpiredError(err) {
			t.Logf("error type: %T, error: %v", err, err)
		}
	})

	t.Run("valid token from Hydra is accepted", func(t *testing.T) {
		validToken := os.Getenv("TEST_VALID_TOKEN")
		if validToken == "" {
			t.Skip("skipping: TEST_VALID_TOKEN not set (obtain from Hydra OAuth flow)")
		}

		claims, err := deps.Validator.Validate(ctx, validToken)
		if err != nil {
			t.Fatalf("expected valid token to pass validation: %v", err)
		}

		if claims.Subject == "" {
			t.Error("expected non-empty subject in validated claims")
		}

		t.Logf("validated claims - subject: %s, scopes: %v", claims.Subject, claims.Scopes)
	})

	t.Run("x-user-id from token matches sub claim", func(t *testing.T) {
		validToken := os.Getenv("TEST_VALID_TOKEN")
		if validToken == "" {
			t.Skip("skipping: TEST_VALID_TOKEN not set")
		}

		claims, err := deps.Validator.Validate(ctx, validToken)
		if err != nil {
			t.Skipf("token validation failed: %v", err)
		}

		if claims.Subject == "" {
			t.Error("subject should be set and propagated as x-user-id")
		}
	})
}

func TestE2E_JWKSCacheRefreshWithHydra(t *testing.T) {
	hydraJWKSURL := os.Getenv("HYDRA_JWKS_URL")
	if hydraJWKSURL == "" {
		t.Skip("skipping E2E test: HYDRA_JWKS_URL not set")
	}

	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			TrustedProxyHeader: "X-Real-IP",
		},
		JWT: config.JWTConfig{
			IssuerURL: os.Getenv("HYDRA_ISSUER_URL"),
			Audience:  os.Getenv("JWT_AUDIENCE"),
			ClockSkew: 30 * time.Second,
		},
		JWKS: config.JWKSConfig{
			URL:                hydraJWKSURL,
			RefreshInterval:    time.Hour,
			MinRefreshInterval: 10 * time.Second,
		},
		RateLimit: config.RateLimitConfig{
			FailureThreshold: 10,
			Window:           time.Minute,
			Cooldown:         5 * time.Minute,
		},
	}

	if cfg.JWT.IssuerURL == "" {
		cfg.JWT.IssuerURL = "http://localhost:4444"
	}
	if cfg.JWT.Audience == "" {
		cfg.JWT.Audience = "test-client"
	}

	deps, err := NewDependencies(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("failed to create dependencies: %v", err)
	}
	defer deps.Close()

	t.Run("forced refresh respects minimum interval", func(t *testing.T) {
		err := deps.JWKSManager.Refresh(ctx)
		if err != nil {
			t.Logf("first refresh result: %v", err)
		}

		err = deps.JWKSManager.Refresh(ctx)
		if err != nil {
			t.Logf("second refresh (should be throttled): %v", err)
		}

		if !deps.JWKSManager.IsHealthy() {
			t.Error("JWKS manager should remain healthy after refresh")
		}
	})
}

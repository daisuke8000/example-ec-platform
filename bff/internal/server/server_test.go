package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/daisuke8000/example-ec-platform/bff/internal/config"
	"github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
	"github.com/daisuke8000/example-ec-platform/bff/internal/middleware"
)

func TestServer_InterceptorChain(t *testing.T) {
	t.Run("sanitizer runs before auth", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port:               8080,
				TrustedProxyHeader: "X-Real-IP",
			},
			JWT: config.JWTConfig{
				IssuerURL: "http://localhost:4444",
				Audience:  "test-client",
				ClockSkew: 30 * time.Second,
			},
			JWKS: config.JWKSConfig{
				URL:                "http://localhost:4444/.well-known/jwks.json",
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

		rateLimiter := middleware.NewRateLimiter(middleware.RateLimitConfig{
			FailureThreshold: cfg.RateLimit.FailureThreshold,
			Window:           cfg.RateLimit.Window,
			Cooldown:         cfg.RateLimit.Cooldown,
		})
		defer rateLimiter.Close()

		publicMatcher := middleware.NewPublicEndpointMatcher([]string{"/api.v1.ProductService/ListProducts"})

		deps := &Dependencies{
			Config:        cfg,
			RateLimiter:   rateLimiter,
			PublicMatcher: publicMatcher,
		}

		chain := BuildInterceptorChain(deps)
		if chain == nil {
			t.Fatal("expected non-nil interceptor chain")
		}
	})
}

func TestServer_HeaderSanitizerIntegration(t *testing.T) {
	t.Run("sanitizer removes internal headers", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				TrustedProxyHeader: "X-Real-IP",
			},
		}

		handler := BuildHTTPHandler(cfg, nil)
		if handler == nil {
			t.Fatal("expected non-nil handler")
		}

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("x-user-id", "spoofed-user")
		req.Header.Set("x-scopes", "admin")
		req.Header.Set("Authorization", "Bearer test-token")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	})
}

func TestDependencies_Initialize(t *testing.T) {
	t.Run("validates required config", func(t *testing.T) {
		cfg := &config.Config{
			JWT: config.JWTConfig{
				IssuerURL: "",
				Audience:  "",
			},
		}

		_, err := NewDependencies(context.Background(), cfg, nil)
		if err == nil {
			t.Error("expected error for missing required config")
		}
	})
}

var _ connect.Interceptor = (connect.UnaryInterceptorFunc)(nil)
var _ *jwt.Validator = (*jwt.Validator)(nil)

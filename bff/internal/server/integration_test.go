package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/daisuke8000/example-ec-platform/bff/internal/config"
	jwtpkg "github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
)

type testJWKS struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	kid        string
	issuer     string
	audience   string
}

func newTestJWKS(t *testing.T) *testJWKS {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	return &testJWKS{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		kid:        "test-key-1",
		issuer:     "http://localhost:4444",
		audience:   "test-client",
	}
}

func (tj *testJWKS) createJWKS() ([]byte, error) {
	key, err := jwk.FromRaw(tj.publicKey)
	if err != nil {
		return nil, err
	}
	_ = key.Set(jwk.KeyIDKey, tj.kid)
	_ = key.Set(jwk.AlgorithmKey, jwa.RS256)
	_ = key.Set(jwk.KeyUsageKey, jwk.ForSignature)

	set := jwk.NewSet()
	_ = set.AddKey(key)

	return json.Marshal(set)
}

func (tj *testJWKS) createToken(userID string, scopes string, expiry time.Duration) (string, error) {
	now := time.Now()

	token, err := jwt.NewBuilder().
		Subject(userID).
		Issuer(tj.issuer).
		Audience([]string{tj.audience}).
		IssuedAt(now).
		Expiration(now.Add(expiry)).
		Claim("scope", scopes).
		Build()
	if err != nil {
		return "", err
	}

	key, err := jwk.FromRaw(tj.privateKey)
	if err != nil {
		return "", err
	}
	_ = key.Set(jwk.KeyIDKey, tj.kid)
	_ = key.Set(jwk.AlgorithmKey, jwa.RS256)

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

func (tj *testJWKS) createExpiredToken(userID string) (string, error) {
	return tj.createToken(userID, "read", -time.Hour)
}

func TestIntegration_AuthenticatedFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testJWKS := newTestJWKS(t)

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwksData, err := testJWKS.createJWKS()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer jwksServer.Close()

	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:               8080,
			TrustedProxyHeader: "X-Real-IP",
		},
		JWT: config.JWTConfig{
			IssuerURL: testJWKS.issuer,
			Audience:  testJWKS.audience,
			ClockSkew: 30 * time.Second,
		},
		JWKS: config.JWKSConfig{
			URL:                jwksServer.URL,
			RefreshInterval:    time.Hour,
			MinRefreshInterval: 10 * time.Second,
		},
		RateLimit: config.RateLimitConfig{
			FailureThreshold: 10,
			Window:           time.Minute,
			Cooldown:         5 * time.Minute,
			Enabled:          true,
		},
		PublicEndpoints: config.PublicEndpointsConfig{
			Endpoints: "/api.v1.ProductService/ListProducts",
		},
	}

	deps, err := NewDependencies(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("failed to create dependencies: %v", err)
	}
	defer deps.Close()

	t.Run("valid token passes authentication", func(t *testing.T) {
		token, err := testJWKS.createToken("user-123", "read write", time.Hour)
		if err != nil {
			t.Fatalf("failed to create token: %v", err)
		}

		claims, err := deps.Validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("expected valid token to pass validation: %v", err)
		}

		if claims.Subject != "user-123" {
			t.Errorf("expected subject user-123, got %s", claims.Subject)
		}

		if len(claims.Scopes) != 2 || claims.Scopes[0] != "read" || claims.Scopes[1] != "write" {
			t.Errorf("expected scopes [read write], got %v", claims.Scopes)
		}
	})

	t.Run("expired token returns TOKEN_EXPIRED error", func(t *testing.T) {
		token, err := testJWKS.createExpiredToken("user-456")
		if err != nil {
			t.Fatalf("failed to create expired token: %v", err)
		}

		_, err = deps.Validator.Validate(ctx, token)
		if err == nil {
			t.Fatal("expected error for expired token")
		}

		if !jwtpkg.IsTokenExpiredError(err) {
			t.Errorf("expected TOKEN_EXPIRED error, got %T: %v", err, err)
		}
	})

	t.Run("public endpoint accessible without token", func(t *testing.T) {
		if !deps.PublicMatcher.IsPublic("/api.v1.ProductService/ListProducts") {
			t.Error("expected /api.v1.ProductService/ListProducts to be public")
		}

		if deps.PublicMatcher.IsPublic("/api.v1.UserService/GetProfile") {
			t.Error("expected /api.v1.UserService/GetProfile to require auth")
		}
	})

	t.Run("header sanitization prevents injection", func(t *testing.T) {
		handler := BuildHTTPHandler(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-user-id") != "" {
				t.Error("x-user-id header should have been sanitized")
			}
			if r.Header.Get("x-scopes") != "" {
				t.Error("x-scopes header should have been sanitized")
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("x-user-id", "spoofed-user")
		req.Header.Set("x-scopes", "admin")
		req.Header.Set("Authorization", "Bearer valid-token")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("rate limiting triggers after threshold", func(t *testing.T) {
		testIP := "192.168.100.50"
		deps.RateLimiter.Reset(testIP)

		for i := 0; i < cfg.RateLimit.FailureThreshold; i++ {
			deps.RateLimiter.RecordFailure(testIP)
		}

		if !deps.RateLimiter.IsRateLimited(testIP) {
			t.Error("expected IP to be rate limited after threshold failures")
		}

		deps.RateLimiter.Reset(testIP)
	})
}

func TestIntegration_ContextPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testJWKS := newTestJWKS(t)

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwksData, _ := testJWKS.createJWKS()
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))
	defer jwksServer.Close()

	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			TrustedProxyHeader: "X-Real-IP",
		},
		JWT: config.JWTConfig{
			IssuerURL: testJWKS.issuer,
			Audience:  testJWKS.audience,
			ClockSkew: 30 * time.Second,
		},
		JWKS: config.JWKSConfig{
			URL:                jwksServer.URL,
			RefreshInterval:    time.Hour,
			MinRefreshInterval: 10 * time.Second,
		},
		RateLimit: config.RateLimitConfig{
			FailureThreshold: 10,
			Window:           time.Minute,
			Cooldown:         5 * time.Minute,
		},
	}

	deps, err := NewDependencies(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("failed to create dependencies: %v", err)
	}
	defer deps.Close()

	t.Run("user context is propagated after validation", func(t *testing.T) {
		token, err := testJWKS.createToken("user-789", "admin superuser", time.Hour)
		if err != nil {
			t.Fatalf("failed to create token: %v", err)
		}

		claims, err := deps.Validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("validation failed: %v", err)
		}

		if claims.Subject != "user-789" {
			t.Errorf("expected user-789, got %s", claims.Subject)
		}

		scopeStr := strings.Join(claims.Scopes, " ")
		if scopeStr != "admin superuser" {
			t.Errorf("expected 'admin superuser', got '%s'", scopeStr)
		}
	})
}

var _ connect.Interceptor = (connect.UnaryInterceptorFunc)(nil)

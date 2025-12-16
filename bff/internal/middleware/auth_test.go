package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	jwtpkg "github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
	"github.com/daisuke8000/example-ec-platform/bff/internal/middleware"
	pkgmw "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
)

type authTestSetup struct {
	privateKey *rsa.PrivateKey
	kid        string
	jwksServer *httptest.Server
	interceptor connect.UnaryInterceptorFunc
}

func setupAuthTest(t *testing.T) *authTestSetup {
	t.Helper()

	kid := "test-kid-auth"
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pubKey, _ := jwk.FromRaw(privateKey.PublicKey)
	_ = pubKey.Set(jwk.KeyIDKey, kid)
	_ = pubKey.Set(jwk.AlgorithmKey, "RS256")
	_ = pubKey.Set(jwk.KeyUsageKey, "sig")

	set := jwk.NewSet()
	_ = set.AddKey(pubKey)
	jwksData, _ := json.Marshal(set)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksData)
	}))

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                server.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	jwksManager, err := jwtpkg.NewJWKSManager(context.Background(), jwksCfg)
	if err != nil {
		t.Fatalf("failed to create JWKS manager: %v", err)
	}

	validatorCfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 30 * time.Second,
	}

	validator := jwtpkg.NewValidator(validatorCfg, jwksManager)

	rateLimiter := middleware.NewRateLimiter(middleware.RateLimitConfig{
		FailureThreshold: 10,
		Window:           time.Minute,
		Cooldown:         5 * time.Minute,
	})

	publicMatcher := middleware.NewPublicEndpointMatcher([]string{
		"/api.v1.ProductService/ListProducts",
	})

	cfg := middleware.AuthInterceptorConfig{
		TrustedProxyHeader: "X-Real-IP",
	}

	interceptor := middleware.NewAuthInterceptor(cfg, validator, rateLimiter, publicMatcher)

	return &authTestSetup{
		privateKey:  privateKey,
		kid:         kid,
		jwksServer:  server,
		interceptor: interceptor,
	}
}

func (s *authTestSetup) signToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()

	builder := jwt.NewBuilder()
	for k, v := range claims {
		builder = builder.Claim(k, v)
	}

	token, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build token: %v", err)
	}

	key, _ := jwk.FromRaw(s.privateKey)
	_ = key.Set(jwk.KeyIDKey, s.kid)
	_ = key.Set(jwk.AlgorithmKey, jwa.RS256)

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return string(signed)
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	setup := setupAuthTest(t)
	defer setup.jwksServer.Close()

	token := setup.signToken(t, map[string]interface{}{
		"iss":   "https://hydra.example.com/",
		"aud":   []string{"test-audience"},
		"sub":   "user-123",
		"scope": "read write",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})

	// Create mock request with authorization header
	ctx := context.Background()
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer "+token)

	// Mock handler that checks context for user info
	var capturedUserID, capturedScopes string
	handler := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		capturedUserID = pkgmw.GetUserID(ctx)
		capturedScopes = pkgmw.GetScopes(ctx)
		return connect.NewResponse(&struct{}{}), nil
	}

	// Apply interceptor
	wrappedHandler := setup.interceptor(handler)
	_, err := wrappedHandler(ctx, req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedUserID != "user-123" {
		t.Errorf("expected user ID 'user-123', got '%s'", capturedUserID)
	}

	if capturedScopes != "read write" {
		t.Errorf("expected scopes 'read write', got '%s'", capturedScopes)
	}
}

func TestAuthInterceptor_PublicEndpoint(t *testing.T) {
	setup := setupAuthTest(t)
	defer setup.jwksServer.Close()

	ctx := context.Background()
	req := connect.NewRequest(&struct{}{})
	// No Authorization header

	// Simulate calling public endpoint
	spec := &connect.Spec{
		Procedure: "/api.v1.ProductService/ListProducts",
	}
	ctx = context.WithValue(ctx, middleware.ProcedureKey{}, spec.Procedure)

	handler := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Should reach handler without auth
		return connect.NewResponse(&struct{}{}), nil
	}

	wrappedHandler := setup.interceptor(handler)
	_, err := wrappedHandler(ctx, req)

	if err != nil {
		t.Errorf("expected public endpoint to succeed without auth, got error: %v", err)
	}
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	setup := setupAuthTest(t)
	defer setup.jwksServer.Close()

	ctx := context.Background()
	req := connect.NewRequest(&struct{}{})
	// No Authorization header

	spec := &connect.Spec{
		Procedure: "/api.v1.UserService/GetUser", // Protected endpoint
	}
	ctx = context.WithValue(ctx, middleware.ProcedureKey{}, spec.Procedure)

	handler := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		t.Error("handler should not be called for missing token")
		return nil, nil
	}

	wrappedHandler := setup.interceptor(handler)
	_, err := wrappedHandler(ctx, req)

	if err == nil {
		t.Error("expected error for missing token")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Errorf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
	}
}

func TestAuthInterceptor_BearerCaseInsensitive(t *testing.T) {
	setup := setupAuthTest(t)
	defer setup.jwksServer.Close()

	token := setup.signToken(t, map[string]interface{}{
		"iss":   "https://hydra.example.com/",
		"aud":   []string{"test-audience"},
		"sub":   "user-123",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	testCases := []string{
		"Bearer " + token,
		"bearer " + token,
		"BEARER " + token,
	}

	for _, authHeader := range testCases {
		t.Run(authHeader[:6], func(t *testing.T) {
			ctx := context.Background()
			req := connect.NewRequest(&struct{}{})
			req.Header().Set("Authorization", authHeader)

			handler := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				return connect.NewResponse(&struct{}{}), nil
			}

			wrappedHandler := setup.interceptor(handler)
			_, err := wrappedHandler(ctx, req)

			if err != nil {
				t.Errorf("expected success with '%s', got error: %v", authHeader[:6], err)
			}
		})
	}
}

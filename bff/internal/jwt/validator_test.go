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

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	jwtpkg "github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
)

type testKeyPair struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	kid        string
	jwksServer *httptest.Server
}

func setupTestKeyPair(t *testing.T, kid string) *testKeyPair {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pubKey, err := jwk.FromRaw(privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to create JWK: %v", err)
	}

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

	return &testKeyPair{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		kid:        kid,
		jwksServer: server,
	}
}

func (kp *testKeyPair) signToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()

	builder := jwt.NewBuilder()
	for k, v := range claims {
		builder = builder.Claim(k, v)
	}

	token, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build token: %v", err)
	}

	key, err := jwk.FromRaw(kp.privateKey)
	if err != nil {
		t.Fatalf("failed to create private JWK: %v", err)
	}
	_ = key.Set(jwk.KeyIDKey, kp.kid)
	_ = key.Set(jwk.AlgorithmKey, jwa.RS256)

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return string(signed)
}

func TestJWTValidator_Validate_Success(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	issuer := "https://hydra.example.com/"
	audience := "test-audience"

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    issuer,
		Audience:  audience,
		ClockSkew: 30 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, err := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	if err != nil {
		t.Fatalf("failed to create JWKS manager: %v", err)
	}
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	token := kp.signToken(t, map[string]interface{}{
		"iss":   issuer,
		"aud":   []string{audience},
		"sub":   "user-123",
		"scope": "read write",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})

	claims, err := validator.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("expected subject 'user-123', got '%s'", claims.Subject)
	}

	if len(claims.Scopes) != 2 || claims.Scopes[0] != "read" || claims.Scopes[1] != "write" {
		t.Errorf("expected scopes [read, write], got %v", claims.Scopes)
	}
}

func TestJWTValidator_Validate_ExpiredToken(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 30 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, _ := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	token := kp.signToken(t, map[string]interface{}{
		"iss":   "https://hydra.example.com/",
		"aud":   []string{"test-audience"},
		"sub":   "user-123",
		"exp":   time.Now().Add(-time.Hour).Unix(), // Expired
		"iat":   time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err := validator.Validate(ctx, token)
	if err == nil {
		t.Error("expected error for expired token")
	}

	if !jwtpkg.IsTokenExpiredError(err) {
		t.Errorf("expected TokenExpiredError, got %T: %v", err, err)
	}
}

func TestJWTValidator_Validate_WrongIssuer(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 30 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, _ := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	token := kp.signToken(t, map[string]interface{}{
		"iss": "https://wrong-issuer.com/",
		"aud": []string{"test-audience"},
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.Validate(ctx, token)
	if err == nil {
		t.Error("expected error for wrong issuer")
	}

	if !jwtpkg.IsInvalidIssuerError(err) {
		t.Errorf("expected InvalidIssuerError, got %T: %v", err, err)
	}
}

func TestJWTValidator_Validate_WrongAudience(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 30 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, _ := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	token := kp.signToken(t, map[string]interface{}{
		"iss": "https://hydra.example.com/",
		"aud": []string{"wrong-audience"},
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.Validate(ctx, token)
	if err == nil {
		t.Error("expected error for wrong audience")
	}

	if !jwtpkg.IsInvalidAudienceError(err) {
		t.Errorf("expected InvalidAudienceError, got %T: %v", err, err)
	}
}

func TestJWTValidator_Validate_ClockSkewTolerance(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 60 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, _ := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	// Token expired 30 seconds ago, but within 60s clock skew
	token := kp.signToken(t, map[string]interface{}{
		"iss": "https://hydra.example.com/",
		"aud": []string{"test-audience"},
		"sub": "user-123",
		"exp": time.Now().Add(-30 * time.Second).Unix(),
		"iat": time.Now().Add(-time.Hour).Unix(),
	})

	claims, err := validator.Validate(ctx, token)
	if err != nil {
		t.Errorf("expected token within clock skew to be valid, got error: %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("expected subject 'user-123', got '%s'", claims.Subject)
	}
}

func TestJWTValidator_Validate_NotBefore(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 30 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, _ := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	// Token not yet valid (nbf in future)
	token := kp.signToken(t, map[string]interface{}{
		"iss": "https://hydra.example.com/",
		"aud": []string{"test-audience"},
		"sub": "user-123",
		"exp": time.Now().Add(2 * time.Hour).Unix(),
		"nbf": time.Now().Add(time.Hour).Unix(), // Not valid yet
	})

	_, err := validator.Validate(ctx, token)
	if err == nil {
		t.Error("expected error for token not yet valid")
	}
}

func TestJWTValidator_ExtractScopes(t *testing.T) {
	kp := setupTestKeyPair(t, "test-kid")
	defer kp.jwksServer.Close()

	cfg := jwtpkg.ValidatorConfig{
		Issuer:    "https://hydra.example.com/",
		Audience:  "test-audience",
		ClockSkew: 30 * time.Second,
	}

	jwksCfg := jwtpkg.JWKSConfig{
		URL:                kp.jwksServer.URL,
		RefreshInterval:    time.Hour,
		MinRefreshInterval: 10 * time.Second,
	}

	ctx := context.Background()
	jwksManager, _ := jwtpkg.NewJWKSManager(ctx, jwksCfg)
	defer jwksManager.Close()

	validator := jwtpkg.NewValidator(cfg, jwksManager)

	tests := []struct {
		name     string
		scope    interface{}
		expected []string
	}{
		{"space_separated", "read write delete", []string{"read", "write", "delete"}},
		{"single_scope", "admin", []string{"admin"}},
		{"empty_scope", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := kp.signToken(t, map[string]interface{}{
				"iss":   "https://hydra.example.com/",
				"aud":   []string{"test-audience"},
				"sub":   "user-123",
				"scope": tt.scope,
				"exp":   time.Now().Add(time.Hour).Unix(),
			})

			claims, err := validator.Validate(ctx, token)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if len(claims.Scopes) != len(tt.expected) {
				t.Errorf("expected %d scopes, got %d", len(tt.expected), len(claims.Scopes))
			}
		})
	}
}

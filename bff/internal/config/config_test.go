package config_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/daisuke8000/example-ec-platform/bff/internal/config"
)

// Helper function to set environment variables and return cleanup function
func setEnv(t *testing.T, envVars map[string]string) func() {
	t.Helper()
	originalValues := make(map[string]string)

	for key, value := range envVars {
		if orig, exists := os.LookupEnv(key); exists {
			originalValues[key] = orig
		}
		os.Setenv(key, value)
	}

	return func() {
		for key := range envVars {
			if orig, exists := originalValues[key]; exists {
				os.Setenv(key, orig)
			} else {
				os.Unsetenv(key)
			}
		}
	}
}

// clearAllEnv clears all config-related environment variables
func clearAllEnv(t *testing.T) func() {
	t.Helper()
	envVars := []string{
		"BFF_PORT",
		"BFF_METRICS_PORT",
		"TRUSTED_PROXY_HEADER",
		"HYDRA_ISSUER_URL",
		"JWT_AUDIENCE",
		"JWT_CLOCK_SKEW",
		"HYDRA_JWKS_URL",
		"JWKS_REFRESH_INTERVAL",
		"JWKS_MIN_REFRESH_INTERVAL",
		"AUTH_RATE_LIMIT_FAILURES",
		"AUTH_RATE_LIMIT_WINDOW",
		"AUTH_RATE_LIMIT_COOLDOWN",
		"AUTH_RATE_LIMIT_ENABLED",
		"PUBLIC_ENDPOINTS",
		"LOG_LEVEL",
		"METRICS_ENABLED",
		"OTEL_SERVICE_NAME",
		"OTEL_SERVICE_VERSION",
		"OTEL_PROMETHEUS_PORT",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	}

	originalValues := make(map[string]string)
	for _, key := range envVars {
		if orig, exists := os.LookupEnv(key); exists {
			originalValues[key] = orig
		}
		os.Unsetenv(key)
	}

	return func() {
		for key, value := range originalValues {
			os.Setenv(key, value)
		}
	}
}

func TestConfig_Load_RequiredFields(t *testing.T) {
	cleanup := clearAllEnv(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Missing required fields should fail
	t.Run("fails_without_required_env_vars", func(t *testing.T) {
		_, err := config.Load(ctx)
		if err == nil {
			t.Error("expected error when required environment variables are missing")
		}
	})

	// Test: With all required fields
	t.Run("succeeds_with_required_env_vars", func(t *testing.T) {
		cleanup := setEnv(t, map[string]string{
			"HYDRA_ISSUER_URL": "http://localhost:4444/",
			"HYDRA_JWKS_URL":   "http://localhost:4444/.well-known/jwks.json",
			"JWT_AUDIENCE":     "ec-platform-bff",
		})
		defer cleanup()

		cfg, err := config.Load(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.JWT.IssuerURL != "http://localhost:4444/" {
			t.Errorf("expected IssuerURL 'http://localhost:4444/', got '%s'", cfg.JWT.IssuerURL)
		}
		if cfg.JWT.Audience != "ec-platform-bff" {
			t.Errorf("expected Audience 'ec-platform-bff', got '%s'", cfg.JWT.Audience)
		}
		if cfg.JWKS.URL != "http://localhost:4444/.well-known/jwks.json" {
			t.Errorf("expected JWKS URL 'http://localhost:4444/.well-known/jwks.json', got '%s'", cfg.JWKS.URL)
		}
	})
}

func TestConfig_Load_DefaultValues(t *testing.T) {
	cleanup := clearAllEnv(t)
	defer cleanup()

	// Set only required fields
	envCleanup := setEnv(t, map[string]string{
		"HYDRA_ISSUER_URL": "http://localhost:4444/",
		"HYDRA_JWKS_URL":   "http://localhost:4444/.well-known/jwks.json",
		"JWT_AUDIENCE":     "ec-platform-bff",
	})
	defer envCleanup()

	ctx := context.Background()
	cfg, err := config.Load(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify default values
	t.Run("server_defaults", func(t *testing.T) {
		if cfg.Server.Port != 8080 {
			t.Errorf("expected default Port 8080, got %d", cfg.Server.Port)
		}
		if cfg.Server.MetricsPort != 8081 {
			t.Errorf("expected default MetricsPort 8081, got %d", cfg.Server.MetricsPort)
		}
		if cfg.Server.TrustedProxyHeader != "X-Real-IP" {
			t.Errorf("expected default TrustedProxyHeader 'X-Real-IP', got '%s'", cfg.Server.TrustedProxyHeader)
		}
	})

	t.Run("jwt_defaults", func(t *testing.T) {
		if cfg.JWT.ClockSkew != 30*time.Second {
			t.Errorf("expected default ClockSkew 30s, got %v", cfg.JWT.ClockSkew)
		}
	})

	t.Run("jwks_defaults", func(t *testing.T) {
		if cfg.JWKS.RefreshInterval != time.Hour {
			t.Errorf("expected default RefreshInterval 1h, got %v", cfg.JWKS.RefreshInterval)
		}
		if cfg.JWKS.MinRefreshInterval != 10*time.Second {
			t.Errorf("expected default MinRefreshInterval 10s, got %v", cfg.JWKS.MinRefreshInterval)
		}
	})

	t.Run("rate_limit_defaults", func(t *testing.T) {
		if cfg.RateLimit.FailureThreshold != 10 {
			t.Errorf("expected default FailureThreshold 10, got %d", cfg.RateLimit.FailureThreshold)
		}
		if cfg.RateLimit.Window != time.Minute {
			t.Errorf("expected default Window 1m, got %v", cfg.RateLimit.Window)
		}
		if cfg.RateLimit.Cooldown != 5*time.Minute {
			t.Errorf("expected default Cooldown 5m, got %v", cfg.RateLimit.Cooldown)
		}
		if !cfg.RateLimit.Enabled {
			t.Error("expected default Enabled true")
		}
	})

	t.Run("observability_defaults", func(t *testing.T) {
		if cfg.Observability.LogLevel != "info" {
			t.Errorf("expected default LogLevel 'info', got '%s'", cfg.Observability.LogLevel)
		}
		if !cfg.Observability.MetricsEnabled {
			t.Error("expected default MetricsEnabled true")
		}
		if cfg.Observability.ServiceName != "bff" {
			t.Errorf("expected default ServiceName 'bff', got '%s'", cfg.Observability.ServiceName)
		}
		if cfg.Observability.ServiceVersion != "unknown" {
			t.Errorf("expected default ServiceVersion 'unknown', got '%s'", cfg.Observability.ServiceVersion)
		}
		if cfg.Observability.PrometheusPort != 9090 {
			t.Errorf("expected default PrometheusPort 9090, got %d", cfg.Observability.PrometheusPort)
		}
	})
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name: "valid_config",
			cfg: config.Config{
				Server: config.ServerConfig{
					Port:        8080,
					MetricsPort: 8081,
				},
				JWT: config.JWTConfig{
					IssuerURL: "http://localhost:4444/",
					Audience:  "ec-platform-bff",
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
				},
				Observability: config.ObservabilityConfig{
					ServiceName:    "bff",
					PrometheusPort: 9090,
				},
			},
			wantErr: false,
		},
		{
			name: "missing_issuer_url",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "missing_audience",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "negative_clock_skew",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: -1 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "missing_jwks_url",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "jwks_refresh_interval_too_short",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: 30 * time.Second, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "min_refresh_interval_too_short",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 5 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_port",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 0, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 10,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
			},
			wantErr: true,
		},
		{
			name: "rate_limit_failure_threshold_zero",
			cfg: config.Config{
				Server: config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:    config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:   config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{
					FailureThreshold: 0,
					Window:           time.Minute,
					Cooldown:         5 * time.Minute,
				},
				Observability: config.ObservabilityConfig{ServiceName: "bff", PrometheusPort: 9090},
			},
			wantErr: true,
		},
		{
			name: "invalid_prometheus_port",
			cfg: config.Config{
				Server:    config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:       config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:      config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{FailureThreshold: 10, Window: time.Minute, Cooldown: 5 * time.Minute},
				Observability: config.ObservabilityConfig{
					ServiceName:    "bff",
					PrometheusPort: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "empty_service_name",
			cfg: config.Config{
				Server:    config.ServerConfig{Port: 8080, MetricsPort: 8081},
				JWT:       config.JWTConfig{IssuerURL: "http://test", Audience: "test", ClockSkew: 30 * time.Second},
				JWKS:      config.JWKSConfig{URL: "http://test", RefreshInterval: time.Hour, MinRefreshInterval: 10 * time.Second},
				RateLimit: config.RateLimitConfig{FailureThreshold: 10, Window: time.Minute, Cooldown: 5 * time.Minute},
				Observability: config.ObservabilityConfig{
					ServiceName:    "",
					PrometheusPort: 9090,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_GetPublicEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty_string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single_endpoint",
			input:    "/api.v1.ProductService/ListProducts",
			expected: []string{"/api.v1.ProductService/ListProducts"},
		},
		{
			name:  "multiple_endpoints",
			input: "/api.v1.ProductService/ListProducts,/api.v1.ProductService/GetProduct",
			expected: []string{
				"/api.v1.ProductService/ListProducts",
				"/api.v1.ProductService/GetProduct",
			},
		},
		{
			name:  "with_whitespace",
			input: " /api.v1.ProductService/ListProducts , /api.v1.ProductService/GetProduct ",
			expected: []string{
				"/api.v1.ProductService/ListProducts",
				"/api.v1.ProductService/GetProduct",
			},
		},
		{
			name:     "empty_after_split",
			input:    ",,",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				PublicEndpoints: config.PublicEndpointsConfig{
					Endpoints: tt.input,
				},
			}

			got := cfg.GetPublicEndpoints()

			if len(got) != len(tt.expected) {
				t.Errorf("GetPublicEndpoints() returned %d endpoints, expected %d", len(got), len(tt.expected))
				return
			}

			for i, endpoint := range got {
				if endpoint != tt.expected[i] {
					t.Errorf("GetPublicEndpoints()[%d] = %s, expected %s", i, endpoint, tt.expected[i])
				}
			}
		})
	}
}

func TestConfig_HeadersToSanitize(t *testing.T) {
	cfg := &config.Config{}

	headers := cfg.HeadersToSanitize()

	// Verify required headers are included
	requiredHeaders := []string{"x-user-id", "x-scopes"}

	for _, required := range requiredHeaders {
		found := false
		for _, h := range headers {
			if h == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("HeadersToSanitize() should include '%s'", required)
		}
	}
}

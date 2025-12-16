package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds all configuration for the BFF JWT verification middleware.
type Config struct {
	// Server configuration
	Server ServerConfig

	// Backend services configuration
	Backend BackendConfig

	// JWT verification configuration
	JWT JWTConfig

	// JWKS cache configuration
	JWKS JWKSConfig

	// Rate limiting configuration
	RateLimit RateLimitConfig

	// Public endpoints configuration
	PublicEndpoints PublicEndpointsConfig

	// Observability configuration
	Observability ObservabilityConfig
}

type BackendConfig struct {
	UserServiceURL    string        `env:"USER_SERVICE_URL,required"`
	ProductServiceURL string        `env:"PRODUCT_SERVICE_URL"`
	OrderServiceURL   string        `env:"ORDER_SERVICE_URL"`
	RequestTimeout    time.Duration `env:"BACKEND_REQUEST_TIMEOUT,default=10s"`
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	// Port is the HTTP port for the BFF server.
	Port int `env:"BFF_PORT,default=8080"`

	// MetricsPort is the port for Prometheus metrics endpoint.
	MetricsPort int `env:"BFF_METRICS_PORT,default=8081"`

	// TrustedProxyHeader is the header to use for client IP extraction.
	// Options: "X-Real-IP", "X-Forwarded-For", or empty for RemoteAddr.
	TrustedProxyHeader string `env:"TRUSTED_PROXY_HEADER,default=X-Real-IP"`
}

// JWTConfig holds JWT verification configuration.
type JWTConfig struct {
	// IssuerURL is the expected JWT issuer (iss claim).
	// Required.
	IssuerURL string `env:"HYDRA_ISSUER_URL,required"`

	// Audience is the expected JWT audience (aud claim).
	// Required.
	Audience string `env:"JWT_AUDIENCE,required"`

	// ClockSkew is the tolerance for exp/nbf claim validation.
	ClockSkew time.Duration `env:"JWT_CLOCK_SKEW,default=30s"`
}

// JWKSConfig holds JWKS cache configuration.
type JWKSConfig struct {
	// URL is the JWKS endpoint URL.
	// Required.
	URL string `env:"HYDRA_JWKS_URL,required"`

	// RefreshInterval is the interval for background JWKS refresh.
	RefreshInterval time.Duration `env:"JWKS_REFRESH_INTERVAL,default=1h"`

	// MinRefreshInterval is the minimum interval between forced refreshes.
	// This prevents DoS attacks via unknown kid forcing frequent refreshes.
	MinRefreshInterval time.Duration `env:"JWKS_MIN_REFRESH_INTERVAL,default=10s"`
}

// RateLimitConfig holds authentication failure rate limiting configuration.
type RateLimitConfig struct {
	// FailureThreshold is the number of failures before rate limiting kicks in.
	FailureThreshold int `env:"AUTH_RATE_LIMIT_FAILURES,default=10"`

	// Window is the time window for counting failures.
	Window time.Duration `env:"AUTH_RATE_LIMIT_WINDOW,default=1m"`

	// Cooldown is the duration to block requests after threshold is exceeded.
	Cooldown time.Duration `env:"AUTH_RATE_LIMIT_COOLDOWN,default=5m"`

	// Enabled controls whether rate limiting is active.
	Enabled bool `env:"AUTH_RATE_LIMIT_ENABLED,default=true"`
}

// PublicEndpointsConfig holds public endpoint whitelist configuration.
type PublicEndpointsConfig struct {
	// Endpoints is a comma-separated list of gRPC full method names
	// that do not require authentication.
	// Example: "/api.v1.ProductService/ListProducts,/api.v1.ProductService/GetProduct"
	Endpoints string `env:"PUBLIC_ENDPOINTS,default="`
}

// ObservabilityConfig holds logging and metrics configuration.
// Uses OpenTelemetry for metrics with Prometheus exporter.
type ObservabilityConfig struct {
	// LogLevel is the logging level (debug, info, warn, error).
	LogLevel string `env:"LOG_LEVEL,default=info"`

	// MetricsEnabled controls whether OpenTelemetry metrics are exposed.
	MetricsEnabled bool `env:"METRICS_ENABLED,default=true"`

	// OTel Resource attributes
	// ServiceName identifies this service in observability backends.
	ServiceName string `env:"OTEL_SERVICE_NAME,default=bff"`

	// ServiceVersion is the version of this service.
	ServiceVersion string `env:"OTEL_SERVICE_VERSION,default=unknown"`

	// PrometheusPort is the port for the Prometheus metrics endpoint.
	// OpenTelemetry metrics are exported in Prometheus format on this port.
	PrometheusPort int `env:"OTEL_PROMETHEUS_PORT,default=9090"`

	// OTLPEndpoint is the endpoint for OTLP exporter (optional).
	// If set, metrics/traces are also sent to an OpenTelemetry Collector.
	// Example: "localhost:4317" or "otel-collector:4317"
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// Load loads configuration from environment variables.
func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate performs validation on the loaded configuration.
func (c *Config) Validate() error {
	var errs []error

	// Validate JWT config
	if c.JWT.IssuerURL == "" {
		errs = append(errs, errors.New("HYDRA_ISSUER_URL is required"))
	}
	if c.JWT.Audience == "" {
		errs = append(errs, errors.New("JWT_AUDIENCE is required"))
	}
	if c.JWT.ClockSkew < 0 {
		errs = append(errs, errors.New("JWT_CLOCK_SKEW must be non-negative"))
	}

	// Validate JWKS config
	if c.JWKS.URL == "" {
		errs = append(errs, errors.New("HYDRA_JWKS_URL is required"))
	}
	if c.JWKS.RefreshInterval < time.Minute {
		errs = append(errs, errors.New("JWKS_REFRESH_INTERVAL must be at least 1 minute"))
	}
	if c.JWKS.MinRefreshInterval < 10*time.Second {
		errs = append(errs, errors.New("JWKS_MIN_REFRESH_INTERVAL must be at least 10 seconds"))
	}

	// Validate rate limit config
	if c.RateLimit.FailureThreshold < 1 {
		errs = append(errs, errors.New("AUTH_RATE_LIMIT_FAILURES must be at least 1"))
	}
	if c.RateLimit.Window < time.Second {
		errs = append(errs, errors.New("AUTH_RATE_LIMIT_WINDOW must be at least 1 second"))
	}
	if c.RateLimit.Cooldown < time.Second {
		errs = append(errs, errors.New("AUTH_RATE_LIMIT_COOLDOWN must be at least 1 second"))
	}

	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, errors.New("BFF_PORT must be between 1 and 65535"))
	}
	if c.Server.MetricsPort < 1 || c.Server.MetricsPort > 65535 {
		errs = append(errs, errors.New("BFF_METRICS_PORT must be between 1 and 65535"))
	}

	// Validate observability config
	if c.Observability.PrometheusPort < 1 || c.Observability.PrometheusPort > 65535 {
		errs = append(errs, errors.New("OTEL_PROMETHEUS_PORT must be between 1 and 65535"))
	}
	if c.Observability.ServiceName == "" {
		errs = append(errs, errors.New("OTEL_SERVICE_NAME must not be empty"))
	}

	// Validate backend config
	if c.Backend.UserServiceURL == "" {
		errs = append(errs, errors.New("USER_SERVICE_URL is required"))
	}
	if c.Backend.RequestTimeout < time.Second {
		errs = append(errs, errors.New("BACKEND_REQUEST_TIMEOUT must be at least 1 second"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// GetPublicEndpoints returns the list of public endpoint patterns.
func (c *Config) GetPublicEndpoints() []string {
	if c.PublicEndpoints.Endpoints == "" {
		return nil
	}

	endpoints := strings.Split(c.PublicEndpoints.Endpoints, ",")
	result := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		trimmed := strings.TrimSpace(ep)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// HeadersToSanitize returns the list of internal headers to remove from incoming requests.
func (c *Config) HeadersToSanitize() []string {
	return []string{
		"x-user-id",
		"x-scopes",
		"x-user-role",
		"x-tenant-id",
	}
}

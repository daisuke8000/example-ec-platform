package observability

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type AuthMetrics struct {
	authLatency            metric.Float64Histogram
	authFailures           metric.Int64Counter
	jwksRefresh            metric.Int64Counter
	rateLimitHits          metric.Int64Counter
	tokenValidationErrors  metric.Int64Counter
	dependencyUp           metric.Int64ObservableGauge
	dependencyStatus       map[string]bool
	dependencyStatusMu     sync.RWMutex
}

func NewAuthMetrics(meter metric.Meter) (*AuthMetrics, error) {
	m := &AuthMetrics{
		dependencyStatus: make(map[string]bool),
	}

	var err error

	m.authLatency, err = meter.Float64Histogram(
		"auth_latency_seconds",
		metric.WithDescription("Authentication processing time in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	m.authFailures, err = meter.Int64Counter(
		"auth_failures_total",
		metric.WithDescription("Total number of authentication failures"),
	)
	if err != nil {
		return nil, err
	}

	m.jwksRefresh, err = meter.Int64Counter(
		"jwks_refresh_total",
		metric.WithDescription("Total number of JWKS cache refresh operations"),
	)
	if err != nil {
		return nil, err
	}

	m.rateLimitHits, err = meter.Int64Counter(
		"ratelimit_hits_total",
		metric.WithDescription("Total number of rate limit hits"),
	)
	if err != nil {
		return nil, err
	}

	m.tokenValidationErrors, err = meter.Int64Counter(
		"token_validation_errors_total",
		metric.WithDescription("Total number of token validation errors by reason"),
	)
	if err != nil {
		return nil, err
	}

	m.dependencyUp, err = meter.Int64ObservableGauge(
		"dependency_up",
		metric.WithDescription("Dependency health status (1=up, 0=down)"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			m.dependencyStatusMu.RLock()
			defer m.dependencyStatusMu.RUnlock()
			for name, up := range m.dependencyStatus {
				val := int64(0)
				if up {
					val = 1
				}
				o.Observe(val, metric.WithAttributes(attribute.String("dependency", name)))
			}
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *AuthMetrics) RecordAuthLatency(ctx context.Context, duration time.Duration, success bool) {
	m.authLatency.Record(ctx, duration.Seconds(),
		metric.WithAttributes(attribute.Bool("success", success)),
	)
}

func (m *AuthMetrics) RecordAuthFailure(ctx context.Context, reason string) {
	m.authFailures.Add(ctx, 1,
		metric.WithAttributes(attribute.String("reason", reason)),
	)
}

func (m *AuthMetrics) RecordJWKSRefresh(ctx context.Context, success bool) {
	m.jwksRefresh.Add(ctx, 1,
		metric.WithAttributes(attribute.Bool("success", success)),
	)
}

func (m *AuthMetrics) RecordRateLimitHit(ctx context.Context) {
	m.rateLimitHits.Add(ctx, 1)
}

func (m *AuthMetrics) RecordTokenValidationError(ctx context.Context, reason string) {
	m.tokenValidationErrors.Add(ctx, 1,
		metric.WithAttributes(attribute.String("reason", reason)),
	)
}

func (m *AuthMetrics) SetDependencyStatus(name string, up bool) {
	m.dependencyStatusMu.Lock()
	defer m.dependencyStatusMu.Unlock()
	m.dependencyStatus[name] = up
}

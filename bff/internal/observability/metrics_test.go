package observability

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestAuthMetrics_RecordAuthLatency(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	metrics, err := NewAuthMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	metrics.RecordAuthLatency(ctx, 150*time.Millisecond, true)
	metrics.RecordAuthLatency(ctx, 300*time.Millisecond, false)

	rm := metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := findMetric(rm, "auth_latency_seconds")
	if found == nil {
		t.Fatal("auth_latency_seconds metric not found")
	}
}

func TestAuthMetrics_RecordAuthFailure(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	metrics, err := NewAuthMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	metrics.RecordAuthFailure(ctx, "token_expired")
	metrics.RecordAuthFailure(ctx, "invalid_signature")
	metrics.RecordAuthFailure(ctx, "token_expired")

	rm := metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := findMetric(rm, "auth_failures_total")
	if found == nil {
		t.Fatal("auth_failures_total metric not found")
	}
}

func TestAuthMetrics_RecordJWKSRefresh(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	metrics, err := NewAuthMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	metrics.RecordJWKSRefresh(ctx, true)
	metrics.RecordJWKSRefresh(ctx, true)
	metrics.RecordJWKSRefresh(ctx, false)

	rm := metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := findMetric(rm, "jwks_refresh_total")
	if found == nil {
		t.Fatal("jwks_refresh_total metric not found")
	}
}

func TestAuthMetrics_RecordRateLimitHit(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	metrics, err := NewAuthMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	metrics.RecordRateLimitHit(ctx)
	metrics.RecordRateLimitHit(ctx)

	rm := metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := findMetric(rm, "ratelimit_hits_total")
	if found == nil {
		t.Fatal("ratelimit_hits_total metric not found")
	}
}

func TestAuthMetrics_SetDependencyStatus(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	metrics, err := NewAuthMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	metrics.SetDependencyStatus("hydra", true)
	metrics.SetDependencyStatus("hydra", false)

	rm := metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := findMetric(rm, "dependency_up")
	if found == nil {
		t.Fatal("dependency_up metric not found")
	}
}

func TestAuthMetrics_RecordTokenValidationError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	metrics, err := NewAuthMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	metrics.RecordTokenValidationError(ctx, "expired")
	metrics.RecordTokenValidationError(ctx, "invalid_signature")

	rm := metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := findMetric(rm, "token_validation_errors_total")
	if found == nil {
		t.Fatal("token_validation_errors_total metric not found")
	}
}

func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

// Compile-time interface check
var _ metric.Meter = (metric.Meter)(nil)
var _ attribute.Key = attribute.Key("")

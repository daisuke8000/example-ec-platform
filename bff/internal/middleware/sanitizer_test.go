package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daisuke8000/example-ec-platform/bff/internal/middleware"
)

func TestHeaderSanitizer_RemovesInternalHeaders(t *testing.T) {
	headers := []string{"x-user-id", "x-scopes", "x-tenant-id"}
	sanitizer := middleware.NewHeaderSanitizer(headers)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that internal headers are removed
		if r.Header.Get("X-User-Id") != "" {
			t.Error("expected X-User-Id to be removed")
		}
		if r.Header.Get("X-Scopes") != "" {
			t.Error("expected X-Scopes to be removed")
		}
		if r.Header.Get("X-Tenant-Id") != "" {
			t.Error("expected X-Tenant-Id to be removed")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := sanitizer.Middleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-User-Id", "attacker-injected")
	req.Header.Set("X-Scopes", "admin superuser")
	req.Header.Set("X-Tenant-Id", "other-tenant")
	req.Header.Set("Authorization", "Bearer valid-token") // Should be preserved

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHeaderSanitizer_PreservesAllowedHeaders(t *testing.T) {
	headers := []string{"x-user-id", "x-scopes"}
	sanitizer := middleware.NewHeaderSanitizer(headers)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authorization should be preserved
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header to be preserved")
		}
		// Content-Type should be preserved
		if r.Header.Get("Content-Type") == "" {
			t.Error("expected Content-Type header to be preserved")
		}
		// X-Request-Id should be preserved
		if r.Header.Get("X-Request-Id") == "" {
			t.Error("expected X-Request-Id header to be preserved")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := sanitizer.Middleware(nextHandler)

	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-123")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHeaderSanitizer_CaseInsensitive(t *testing.T) {
	headers := []string{"x-user-id"}
	sanitizer := middleware.NewHeaderSanitizer(headers)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// All case variations should be removed
		if r.Header.Get("X-User-Id") != "" {
			t.Error("expected x-user-id to be removed (case insensitive)")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := sanitizer.Middleware(nextHandler)

	// Test various case combinations
	cases := []string{"X-User-Id", "x-user-id", "X-USER-ID", "x-User-Id"}

	for _, headerCase := range cases {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set(headerCase, "injected-value")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func TestHeaderSanitizer_EmptyHeaderList(t *testing.T) {
	sanitizer := middleware.NewHeaderSanitizer([]string{})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// All headers should pass through
		if r.Header.Get("X-Custom-Header") == "" {
			t.Error("expected custom header to be preserved")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := sanitizer.Middleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Custom-Header", "value")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

package middleware_test

import (
	"testing"

	"github.com/daisuke8000/example-ec-platform/bff/internal/middleware"
)

func TestPublicEndpointMatcher_ExactMatch(t *testing.T) {
	endpoints := []string{
		"/api.v1.ProductService/ListProducts",
		"/api.v1.ProductService/GetProduct",
	}

	matcher := middleware.NewPublicEndpointMatcher(endpoints)

	tests := []struct {
		procedure string
		expected  bool
	}{
		{"/api.v1.ProductService/ListProducts", true},
		{"/api.v1.ProductService/GetProduct", true},
		{"/api.v1.ProductService/CreateProduct", false},
		{"/api.v1.UserService/GetUser", false},
	}

	for _, tt := range tests {
		t.Run(tt.procedure, func(t *testing.T) {
			result := matcher.IsPublic(tt.procedure)
			if result != tt.expected {
				t.Errorf("IsPublic(%s) = %v, expected %v", tt.procedure, result, tt.expected)
			}
		})
	}
}

func TestPublicEndpointMatcher_NoWildcards(t *testing.T) {
	endpoints := []string{
		"/api.v1.ProductService/ListProducts",
	}

	matcher := middleware.NewPublicEndpointMatcher(endpoints)

	// Prefix should not match
	if matcher.IsPublic("/api.v1.ProductService/List") {
		t.Error("prefix should not match (no wildcards)")
	}

	// Suffix should not match
	if matcher.IsPublic("/api.v1.ProductService/ListProductsExtra") {
		t.Error("suffix should not match (no wildcards)")
	}
}

func TestPublicEndpointMatcher_EmptyWhitelist(t *testing.T) {
	matcher := middleware.NewPublicEndpointMatcher([]string{})

	// All endpoints should require auth
	if matcher.IsPublic("/api.v1.ProductService/ListProducts") {
		t.Error("empty whitelist should require auth for all endpoints")
	}

	if matcher.IsPublic("/any/endpoint") {
		t.Error("empty whitelist should require auth for all endpoints")
	}
}

func TestPublicEndpointMatcher_NilWhitelist(t *testing.T) {
	matcher := middleware.NewPublicEndpointMatcher(nil)

	if matcher.IsPublic("/api.v1.ProductService/ListProducts") {
		t.Error("nil whitelist should require auth for all endpoints")
	}
}

func TestPublicEndpointMatcher_GRPCMethodFormat(t *testing.T) {
	endpoints := []string{
		"/api.v1.ProductService/ListProducts",
	}

	matcher := middleware.NewPublicEndpointMatcher(endpoints)

	// Valid gRPC format
	if !matcher.IsPublic("/api.v1.ProductService/ListProducts") {
		t.Error("valid gRPC format should match")
	}

	// Missing leading slash
	if matcher.IsPublic("api.v1.ProductService/ListProducts") {
		t.Error("missing leading slash should not match")
	}
}

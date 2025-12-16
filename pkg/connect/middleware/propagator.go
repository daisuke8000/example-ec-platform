// Package middleware provides Connect-go interceptors for cross-service communication.
package middleware

import (
	"context"

	"connectrpc.com/connect"
)

// Metadata header keys for downstream service communication.
const (
	// MetadataUserID is the header key for user identifier (from JWT sub claim).
	MetadataUserID = "x-user-id"

	// MetadataScopes is the header key for user scopes (space-separated).
	MetadataScopes = "x-scopes"

	// MetadataRequestID is the header key for request correlation ID.
	MetadataRequestID = "x-request-id"
)

// Context keys for user information.
type userIDKey struct{}
type scopesKey struct{}
type requestIDKey struct{}

// GetUserID retrieves the user ID from context.
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(userIDKey{}); v != nil {
		return v.(string)
	}
	return ""
}

// GetScopes retrieves the scopes from context as space-separated string.
func GetScopes(ctx context.Context) string {
	if v := ctx.Value(scopesKey{}); v != nil {
		return v.(string)
	}
	return ""
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey{}); v != nil {
		return v.(string)
	}
	return ""
}

// WithUserID adds a user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// WithScopes adds scopes to the context.
func WithScopes(ctx context.Context, scopes string) context.Context {
	return context.WithValue(ctx, scopesKey{}, scopes)
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// InjectUserContext creates a context with user information.
// This is a convenience function for testing and manual context creation.
func InjectUserContext(ctx context.Context, userID, scopes string) context.Context {
	ctx = context.WithValue(ctx, userIDKey{}, userID)
	ctx = context.WithValue(ctx, scopesKey{}, scopes)
	return ctx
}

// ExtractUserContext extracts user information from incoming request headers.
// This is used by downstream services to read propagated user context.
// Returns empty strings if headers are not present.
func ExtractUserContext(req connect.AnyRequest) (userID, scopes, requestID string) {
	userID = req.Header().Get(MetadataUserID)
	scopes = req.Header().Get(MetadataScopes)
	requestID = req.Header().Get(MetadataRequestID)
	return
}

// ContextPropagator injects validated user context into outgoing gRPC metadata
// for downstream service communication.
type ContextPropagator struct{}

// NewContextPropagator creates a new context propagator.
func NewContextPropagator() *ContextPropagator {
	return &ContextPropagator{}
}

// ClientPropagatorInterceptor creates a Connect-go client interceptor that propagates
// user context to downstream services via gRPC metadata headers.
//
// This interceptor should be applied to gRPC clients used by the BFF to call
// backend services. It reads user information from the context (set by
// AuthInterceptor) and injects it into outgoing request headers.
func (p *ContextPropagator) ClientPropagatorInterceptor() connect.UnaryInterceptorFunc {
	return ClientPropagatorInterceptor()
}

// ClientPropagatorInterceptor creates a Connect-go client interceptor that propagates
// user context to downstream services via gRPC metadata headers.
func ClientPropagatorInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Extract user info from context (set by AuthInterceptor)
			userID := GetUserID(ctx)
			scopes := GetScopes(ctx)
			requestID := GetRequestID(ctx)

			// Only inject metadata if user is authenticated
			if userID != "" {
				req.Header().Set(MetadataUserID, userID)
			}

			if scopes != "" {
				req.Header().Set(MetadataScopes, scopes)
			}

			// Always propagate request ID if present (for distributed tracing)
			if requestID != "" {
				req.Header().Set(MetadataRequestID, requestID)
			}

			return next(ctx, req)
		}
	}
}

// ServerPropagatorInterceptor creates a Connect-go server interceptor that
// extracts propagated user context from incoming requests and injects it
// into the Go context for use by service handlers.
//
// This interceptor should be applied to backend services that receive
// requests from the BFF with propagated user context.
func ServerPropagatorInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Extract user context from headers
			userID, scopes, requestID := ExtractUserContext(req)

			// Inject into context
			if userID != "" {
				ctx = context.WithValue(ctx, userIDKey{}, userID)
			}

			if scopes != "" {
				ctx = context.WithValue(ctx, scopesKey{}, scopes)
			}

			if requestID != "" {
				ctx = context.WithValue(ctx, requestIDKey{}, requestID)
			}

			return next(ctx, req)
		}
	}
}

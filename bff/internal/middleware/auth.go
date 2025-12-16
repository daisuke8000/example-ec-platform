package middleware

import (
	"context"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	"github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
	pkgmw "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
)

// ProcedureKey is used to retrieve procedure name from context.
type ProcedureKey struct{}

// AuthInterceptorConfig holds configuration for the auth interceptor.
type AuthInterceptorConfig struct {
	// TrustedProxyHeader is the header to extract client IP from (e.g., X-Real-IP, X-Forwarded-For).
	TrustedProxyHeader string
}

// NewAuthInterceptor creates a Connect-go unary interceptor for JWT authentication.
// It validates Bearer tokens, checks rate limits, and propagates user context.
func NewAuthInterceptor(
	cfg AuthInterceptorConfig,
	validator *jwt.Validator,
	rateLimiter *RateLimiter,
	publicMatcher *PublicEndpointMatcher,
) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Get procedure name from context or request spec
			procedure := getProcedure(ctx, req)

			// Check if endpoint is public
			if publicMatcher.IsPublic(procedure) {
				return next(ctx, req)
			}

			// Extract client IP for rate limiting
			clientIP := extractClientIP(req, cfg.TrustedProxyHeader)

			// Check rate limit before processing
			if rateLimiter.IsRateLimited(clientIP) {
				slog.Warn("rate limited",
					"client_ip", clientIP,
					"procedure", procedure,
				)
				return nil, connect.NewError(
					connect.CodeResourceExhausted,
					nil,
				)
			}

			// Extract Bearer token
			token, err := extractBearerToken(req)
			if err != nil {
				recordFailureAndLog(rateLimiter, clientIP, procedure, "missing_token")
				return nil, newUnauthenticatedError()
			}

			// Validate JWT
			claims, err := validator.Validate(ctx, token)
			if err != nil {
				reason := categorizeValidationError(err)
				recordFailureAndLog(rateLimiter, clientIP, procedure, reason)
				return nil, newUnauthenticatedError()
			}

			// Inject user context using shared package for consistent context keys
			ctx = pkgmw.WithUserID(ctx, claims.Subject)
			ctx = pkgmw.WithScopes(ctx, strings.Join(claims.Scopes, " "))

			slog.Debug("authentication successful",
				"user_id", claims.Subject,
				"procedure", procedure,
			)

			return next(ctx, req)
		}
	}
}

// getProcedure extracts procedure name from context or request.
func getProcedure(ctx context.Context, req connect.AnyRequest) string {
	// First, check context (used in tests)
	if v := ctx.Value(ProcedureKey{}); v != nil {
		if procedure, ok := v.(string); ok {
			return procedure
		}
	}

	// Then, try to get from request spec
	if req.Spec().Procedure != "" {
		return req.Spec().Procedure
	}

	return ""
}

// extractBearerToken extracts the token from Authorization header.
// Supports case-insensitive Bearer scheme (Bearer, bearer, BEARER).
func extractBearerToken(req connect.AnyRequest) (string, error) {
	authHeader := req.Header().Get("Authorization")
	if authHeader == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Case-insensitive Bearer check
	if len(authHeader) < 7 {
		return "", connect.NewError(connect.CodeUnauthenticated, nil)
	}

	prefix := strings.ToLower(authHeader[:6])
	if prefix != "bearer" {
		return "", connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Check for space after Bearer
	if authHeader[6] != ' ' {
		return "", connect.NewError(connect.CodeUnauthenticated, nil)
	}

	token := strings.TrimSpace(authHeader[7:])
	if token == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, nil)
	}

	return token, nil
}

// extractClientIP extracts client IP from request headers.
func extractClientIP(req connect.AnyRequest, trustedHeader string) string {
	if trustedHeader != "" {
		if ip := req.Header().Get(trustedHeader); ip != "" {
			// Handle X-Forwarded-For format (comma-separated IPs, first is client)
			if idx := strings.Index(ip, ","); idx != -1 {
				return strings.TrimSpace(ip[:idx])
			}
			return strings.TrimSpace(ip)
		}
	}

	// Fallback to peer address if available
	if peer := req.Peer(); peer.Addr != "" {
		// Remove port from address
		addr := peer.Addr
		if idx := strings.LastIndex(addr, ":"); idx != -1 {
			return addr[:idx]
		}
		return addr
	}

	return "unknown"
}

// newUnauthenticatedError creates a generic unauthenticated error.
// Does not reveal specific failure reason to client.
func newUnauthenticatedError() *connect.Error {
	err := connect.NewError(connect.CodeUnauthenticated, nil)
	// Add WWW-Authenticate header as per requirements
	if md := err.Meta(); md != nil {
		md.Set("WWW-Authenticate", "Bearer")
	}
	return err
}

// recordFailureAndLog records auth failure and logs it.
func recordFailureAndLog(rateLimiter *RateLimiter, clientIP, procedure, reason string) {
	nowRateLimited := rateLimiter.RecordFailure(clientIP)
	slog.Warn("authentication failed",
		"reason", reason,
		"client_ip", clientIP,
		"procedure", procedure,
		"rate_limited", nowRateLimited,
	)
}

// categorizeValidationError maps validation errors to reason strings.
func categorizeValidationError(err error) string {
	switch {
	case jwt.IsTokenExpiredError(err):
		return "token_expired"
	case jwt.IsInvalidIssuerError(err):
		return "invalid_issuer"
	case jwt.IsInvalidAudienceError(err):
		return "invalid_audience"
	case jwt.IsInvalidSignatureError(err):
		return "invalid_signature"
	case jwt.IsInvalidAlgorithmError(err):
		return "invalid_algorithm"
	default:
		return "validation_failed"
	}
}


package middleware

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

// LoggingInterceptor creates a Connect-go interceptor for request logging.
// It logs the procedure name, duration, and any errors that occur.
func LoggingInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()

			resp, err := next(ctx, req)

			duration := time.Since(start)

			// Extract request ID for correlation
			requestID := GetRequestID(ctx)

			if err != nil {
				logger.ErrorContext(ctx, "RPC failed",
					slog.String("procedure", req.Spec().Procedure),
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
					slog.String("request_id", requestID),
				)
			} else {
				logger.InfoContext(ctx, "RPC completed",
					slog.String("procedure", req.Spec().Procedure),
					slog.Duration("duration", duration),
					slog.String("request_id", requestID),
				)
			}

			return resp, err
		}
	}
}

// DebugLoggingInterceptor creates a more verbose logging interceptor
// that includes additional request details. Use only in development.
func DebugLoggingInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()

			// Log request details
			requestID := GetRequestID(ctx)
			userID := GetUserID(ctx)

			logger.DebugContext(ctx, "RPC started",
				slog.String("procedure", req.Spec().Procedure),
				slog.String("request_id", requestID),
				slog.String("user_id", userID),
				slog.String("peer", req.Peer().Addr),
			)

			resp, err := next(ctx, req)

			duration := time.Since(start)

			if err != nil {
				logger.ErrorContext(ctx, "RPC failed",
					slog.String("procedure", req.Spec().Procedure),
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
					slog.String("request_id", requestID),
				)
			} else {
				logger.DebugContext(ctx, "RPC completed",
					slog.String("procedure", req.Spec().Procedure),
					slog.Duration("duration", duration),
					slog.String("request_id", requestID),
				)
			}

			return resp, err
		}
	}
}

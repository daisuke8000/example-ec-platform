// Package http provides HTTP server infrastructure for OAuth2 UI endpoints.
package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Server represents the HTTP server for OAuth2 UI endpoints.
type Server struct {
	server *http.Server
	logger *slog.Logger
}

// Config holds HTTP server configuration.
type Config struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DefaultConfig returns default HTTP server configuration.
func DefaultConfig(port int) Config {
	return Config{
		Port:            port,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// NewServer creates a new HTTP server with the provided handler and configuration.
func NewServer(handler http.Handler, cfg Config, logger *slog.Logger) *Server {
	return &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		logger: logger,
	}
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting", slog.String("address", s.server.Addr))
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("HTTP server shutting down")
	return s.server.Shutdown(ctx)
}

// SecurityHeadersMiddleware adds security headers to all responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// NewCrossOriginProtection creates a CrossOriginProtection instance
// configured for OAuth2 UI endpoints.
func NewCrossOriginProtection(trustedOrigins []string) *http.CrossOriginProtection {
	corp := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		_ = corp.AddTrustedOrigin(origin)
	}
	// Allow GET endpoints without CSRF protection (they don't change state)
	corp.AddInsecureBypassPattern("GET /")
	corp.AddInsecureBypassPattern("GET /health")
	corp.AddInsecureBypassPattern("GET /healthz")
	corp.AddInsecureBypassPattern("GET /readyz")
	return corp
}

// LoggingMiddleware logs HTTP requests.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			logger.Info("HTTP request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", time.Since(start)),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

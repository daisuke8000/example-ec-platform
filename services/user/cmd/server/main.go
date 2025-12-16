// Package main provides the entry point for the User Service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"
	pkgmiddleware "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
	connectHandler "github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/connect"
	httpAdapter "github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/http"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/hydra"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/ratelimit"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/repository"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/config"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/usecase"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Info("configuration loaded",
		slog.Int("grpc_port", cfg.GRPCPort),
		slog.Int("http_port", cfg.HTTPPort),
	)

	// Initialize database connection pool
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}
	defer pool.Close()

	// Verify database connectivity
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	logger.Info("database connection established")

	// Wire dependencies
	userRepo := repository.NewPostgresUserRepository(pool)
	userUseCase := usecase.NewUserUseCase(userRepo, cfg.BcryptCost)
	userHandler := connectHandler.NewUserServiceHandler(userUseCase, logger)

	// Initialize Redis client for rate limiting (optional - graceful fallback if unavailable)
	var rateLimiter httpAdapter.RateLimiter
	if cfg.RedisURL != "" {
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Warn("failed to parse Redis URL, rate limiting disabled", slog.String("error", err.Error()))
		} else {
			redisClient := redis.NewClient(redisOpts)
			// Test Redis connectivity
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Warn("failed to connect to Redis, rate limiting disabled", slog.String("error", err.Error()))
				redisClient.Close()
			} else {
				logger.Info("Redis connection established for rate limiting")
				rateLimiter = ratelimit.NewRedisRateLimiter(redisClient, ratelimit.DefaultConfig())
				defer redisClient.Close()
			}
		}
	} else {
		logger.Info("Redis URL not configured, rate limiting disabled")
	}

	// Initialize Hydra client
	hydraClient := hydra.NewClient(cfg.HydraAdminURL)
	logger.Info("Hydra client initialized", slog.String("admin_url", cfg.HydraAdminURL))

	// Create HTTP handler for OAuth2 UI
	oauth2Handler, err := httpAdapter.NewHandler(hydraClient, userUseCase, rateLimiter, logger, httpAdapter.HandlerConfig{
		LoginRememberFor:   cfg.LoginRememberFor,
		ConsentRememberFor: cfg.ConsentRememberFor,
	})
	if err != nil {
		return fmt.Errorf("failed to create HTTP handler: %w", err)
	}

	// Create Connect-go interceptors
	interceptors := connect.WithInterceptors(
		pkgmiddleware.ServerPropagatorInterceptor(),
		pkgmiddleware.LoggingInterceptor(logger),
	)

	// Create Connect-go handler
	path, handler := userv1connect.NewUserServiceHandler(userHandler, interceptors)

	// Create combined HTTP mux
	mux := http.NewServeMux()

	// Mount Connect-go handler (handles /user.v1.UserService/*)
	mux.Handle(path, handler)

	// Mount OAuth2 handlers (handles /oauth2/*, /health)
	mux.Handle("/oauth2/", oauth2Handler.Router())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Add health check endpoint for Connect-go service (Kubernetes compatible)
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", handleReadyz(pool))

	// Apply cross-origin protection and security headers
	corp := httpAdapter.NewCrossOriginProtection(cfg.TrustedOrigins)
	wrappedHandler := corp.Handler(
		httpAdapter.SecurityHeadersMiddleware(
			httpAdapter.LoggingMiddleware(logger)(mux),
		),
	)

	// Create HTTP server with h2c (HTTP/2 over cleartext) support
	// This enables HTTP/2 without TLS for gRPC compatibility
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	server := &http.Server{
		Addr: grpcAddr,
		Handler: h2c.NewHandler(
			wrappedHandler,
			&http2.Server{},
		),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)

	// Start server
	go func() {
		logger.Info("Connect-go server starting",
			slog.String("address", grpcAddr),
			slog.String("protocols", "Connect, gRPC, gRPC-Web, HTTP/1.1, HTTP/2"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))
	case err := <-errCh:
		return err
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
	} else {
		logger.Info("server stopped")
	}

	return nil
}

// handleHealthz returns OK if the service is running (liveness probe).
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "serving",
	})
}

// handleReadyz returns OK if the service is ready to accept traffic (readiness probe).
func handleReadyz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check database connectivity
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "not_ready",
				"reason": "database connection failed",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ready",
		})
	}
}

// Package main provides the entry point for the User Service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	pb "github.com/daisuke8000/example-ec-platform/gen/user/v1"
	grpcHandler "github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/grpc"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/http"
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
	userHandler := grpcHandler.NewUserServiceHandler(userUseCase, logger)

	// Initialize Redis client for rate limiting (optional - graceful fallback if unavailable)
	var rateLimiter http.RateLimiter
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
	httpHandler, err := http.NewHandler(hydraClient, userUseCase, rateLimiter, logger, http.HandlerConfig{
		LoginRememberFor:   cfg.LoginRememberFor,
		ConsentRememberFor: cfg.ConsentRememberFor,
	})
	if err != nil {
		return fmt.Errorf("failed to create HTTP handler: %w", err)
	}

	// Create HTTP server with security middleware
	corp := http.NewCrossOriginProtection(cfg.TrustedOrigins)
	httpServerCfg := http.DefaultConfig(cfg.HTTPPort)
	httpServer := http.NewServer(
		corp.Handler(
			http.SecurityHeadersMiddleware(
				http.LoggingMiddleware(logger)(httpHandler.Router()),
			),
		),
		httpServerCfg,
		logger,
	)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register services
	pb.RegisterUserServiceServer(grpcServer, userHandler)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("user.v1.UserService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable server reflection for development
	reflection.Register(grpcServer)

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", grpcAddr, err)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 2)

	// Start gRPC server in goroutine
	go func() {
		logger.Info("gRPC server starting", slog.String("address", grpcAddr))
		if err := grpcServer.Serve(grpcListener); err != nil {
			errCh <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Start HTTP server in goroutine
	go func() {
		if err := httpServer.Start(); err != nil {
			errCh <- err
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

	// Shutdown HTTP server first (quick)
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
	} else {
		logger.Info("HTTP server stopped")
	}

	// Gracefully stop gRPC server
	grpcServer.GracefulStop()
	logger.Info("gRPC server stopped")

	return nil
}

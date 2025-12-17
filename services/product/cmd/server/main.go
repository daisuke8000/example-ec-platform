package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/daisuke8000/example-ec-platform/gen/product/v1/productv1connect"
	pkgmiddleware "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
	connectHandler "github.com/daisuke8000/example-ec-platform/services/product/internal/adapter/connect"
	redisAdapter "github.com/daisuke8000/example-ec-platform/services/product/internal/adapter/redis"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/adapter/repository"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/config"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/usecase"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/worker"
)

func main() {
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

	cfg, err := config.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Info("configuration loaded",
		slog.String("service", cfg.ServiceName),
		slog.Int("grpc_port", cfg.GRPCPort),
		slog.Duration("reservation_ttl", cfg.ReservationTTL),
		slog.Duration("ttl_worker_interval", cfg.TTLWorkerInterval),
	)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	logger.Info("database connection established")

	var idempotencyStore usecase.IdempotencyStore
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Warn("failed to parse Redis URL, idempotency disabled", slog.String("error", err.Error()))
		} else {
			redisClient = redis.NewClient(redisOpts)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Warn("failed to connect to Redis, idempotency disabled", slog.String("error", err.Error()))
				redisClient.Close()
				redisClient = nil
			} else {
				logger.Info("Redis connection established")
				idempotencyStore = redisAdapter.NewIdempotencyStore(redisClient, "product:idempotency:")
			}
		}
	} else {
		logger.Warn("Redis URL not configured, idempotency disabled")
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	if idempotencyStore == nil {
		idempotencyStore = redisAdapter.NewNoopIdempotencyStore()
		logger.Warn("using no-op idempotency store")
	}

	txManager := repository.NewTxManager(pool)
	productRepo := repository.NewPostgresProductRepository(pool)
	skuRepo := repository.NewPostgresSKURepository(pool)
	categoryRepo := repository.NewPostgresCategoryRepository(pool)
	inventoryRepo := repository.NewPostgresInventoryRepository(pool)
	reservationRepo := repository.NewPostgresReservationRepository(pool)

	productUC := usecase.NewProductUseCase(productRepo, categoryRepo)
	skuUC := usecase.NewSKUUseCase(skuRepo, productRepo, inventoryRepo)
	categoryUC := usecase.NewCategoryUseCase(categoryRepo)
	inventoryUC := usecase.NewInventoryUseCase(
		inventoryRepo,
		reservationRepo,
		idempotencyStore,
		txManager,
		cfg.MaxBatchSize,
		cfg.ReservationTTL,
		cfg.IdempotencyKeyTTL,
	)

	productHandler := connectHandler.NewProductHandler(productUC, skuUC, categoryUC)
	inventoryHandler := connectHandler.NewInventoryHandler(inventoryUC)

	interceptors := connect.WithInterceptors(
		pkgmiddleware.ServerPropagatorInterceptor(),
		pkgmiddleware.LoggingInterceptor(logger),
	)

	mux := http.NewServeMux()

	productPath, productSvcHandler := productv1connect.NewProductServiceHandler(productHandler, interceptors)
	mux.Handle(productPath, productSvcHandler)

	inventoryPath, inventorySvcHandler := productv1connect.NewInventoryServiceHandler(inventoryHandler, interceptors)
	mux.Handle(inventoryPath, inventorySvcHandler)

	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", handleReadyz(pool, redisClient, logger))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	server := &http.Server{
		Addr:         grpcAddr,
		Handler:      h2c.NewHandler(mux, &http2.Server{}),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)

	var wg sync.WaitGroup
	workerCtx, workerCancel := context.WithCancel(ctx)
	expirer := worker.NewReservationExpirer(
		txManager,
		reservationRepo,
		inventoryRepo,
		logger.With("component", "reservation-expirer"),
		cfg.TTLWorkerInterval,
		cfg.TTLWorkerBatchSize,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		expirer.Start(workerCtx)
	}()

	go func() {
		logger.Info("server starting",
			slog.String("address", grpcAddr),
			slog.String("protocols", "Connect, gRPC, gRPC-Web"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))
	case err := <-errCh:
		return err
	}

	logger.Info("initiating graceful shutdown")

	workerCancel()
	wg.Wait()
	logger.Info("reservation expirer stopped")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
	} else {
		logger.Info("server stopped")
	}

	return nil
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "serving"})
}

// handleReadyz checks database (required) and Redis (optional, degraded mode allowed).
func handleReadyz(pool *pgxpool.Pool, redisClient *redis.Client, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "not_ready",
				"reason": "database connection failed",
			})
			return
		}

		redisStatus := "not_configured"
		if redisClient != nil {
			if err := redisClient.Ping(r.Context()).Err(); err != nil {
				redisStatus = "degraded"
				logger.Warn("Redis health check failed", slog.String("error", err.Error()))
			} else {
				redisStatus = "healthy"
			}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ready",
			"redis":  redisStatus,
		})
	}
}

package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	ServiceName        string        `env:"SERVICE_NAME,default=product-service"`
	LogLevel           string        `env:"LOG_LEVEL,default=info"`
	GRPCPort           int           `env:"GRPC_PORT,default=50052"`
	DatabaseURL        string        `env:"DATABASE_URL,required"`
	RedisURL           string        `env:"REDIS_URL"`
	ReservationTTL     time.Duration `env:"RESERVATION_TTL,default=15m"`
	TTLWorkerInterval  time.Duration `env:"TTL_WORKER_INTERVAL,default=30s"`
	TTLWorkerBatchSize int           `env:"TTL_WORKER_BATCH_SIZE,default=100"`
	MaxBatchSize       int           `env:"MAX_BATCH_SIZE,default=50"`
	IdempotencyKeyTTL  time.Duration `env:"IDEMPOTENCY_KEY_TTL,default=24h"`
}

func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.MaxBatchSize < 1 || c.MaxBatchSize > 100 {
		return fmt.Errorf("max batch size must be between 1 and 100, got %d", c.MaxBatchSize)
	}

	if c.ReservationTTL < time.Minute || c.ReservationTTL > time.Hour {
		return fmt.Errorf("reservation TTL must be between 1 minute and 1 hour, got %v", c.ReservationTTL)
	}

	if c.TTLWorkerInterval < 10*time.Second || c.TTLWorkerInterval > 5*time.Minute {
		return fmt.Errorf("TTL worker interval must be between 10 seconds and 5 minutes, got %v", c.TTLWorkerInterval)
	}

	return nil
}

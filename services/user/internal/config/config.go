package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	GRPCPort    int    `env:"GRPC_PORT,default=50051"`
	HTTPPort    int    `env:"HTTP_PORT,default=8051"`
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,default=localhost:6379"`

	HydraAdminURL string `env:"HYDRA_ADMIN_URL,required"`

	BcryptCost int `env:"BCRYPT_COST,default=10"`

	LoginRateLimitAttempts int           `env:"LOGIN_RATE_LIMIT_ATTEMPTS,default=5"`
	LoginRateLimitWindow   time.Duration `env:"LOGIN_RATE_LIMIT_WINDOW,default=15m"`

	// Session duration when "Remember Me" is checked (in seconds)
	LoginRememberFor   int `env:"LOGIN_REMEMBER_FOR,default=604800"`   // 7 days
	ConsentRememberFor int `env:"CONSENT_REMEMBER_FOR,default=2592000"` // 30 days

	// CSRF protection: trusted origins for cross-origin requests
	TrustedOrigins []string `env:"TRUSTED_ORIGINS"`
}

func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.BcryptCost < 4 || cfg.BcryptCost > 31 {
		return nil, fmt.Errorf("bcrypt cost must be between 4 and 31, got %d", cfg.BcryptCost)
	}

	return &cfg, nil
}

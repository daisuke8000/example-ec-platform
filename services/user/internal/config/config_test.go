package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Save original env vars and restore after test
	originalDBURL := os.Getenv("DATABASE_URL")
	originalHydraURL := os.Getenv("HYDRA_ADMIN_URL")
	defer func() {
		os.Setenv("DATABASE_URL", originalDBURL)
		os.Setenv("HYDRA_ADMIN_URL", originalHydraURL)
	}()

	tests := []struct {
		name        string
		envVars     map[string]string
		wantErr     bool
		checkConfig func(*testing.T, *Config)
	}{
		{
			name: "loads required fields successfully",
			envVars: map[string]string{
				"DATABASE_URL":    "postgres://user:pass@localhost:5432/db",
				"HYDRA_ADMIN_URL": "http://localhost:4445",
			},
			wantErr: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
					t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://user:pass@localhost:5432/db")
				}
				if cfg.HydraAdminURL != "http://localhost:4445" {
					t.Errorf("HydraAdminURL = %q, want %q", cfg.HydraAdminURL, "http://localhost:4445")
				}
			},
		},
		{
			name: "uses default values",
			envVars: map[string]string{
				"DATABASE_URL":    "postgres://localhost/db",
				"HYDRA_ADMIN_URL": "http://localhost:4445",
			},
			wantErr: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				if cfg.GRPCPort != 50051 {
					t.Errorf("GRPCPort = %d, want %d", cfg.GRPCPort, 50051)
				}
				if cfg.HTTPPort != 8051 {
					t.Errorf("HTTPPort = %d, want %d", cfg.HTTPPort, 8051)
				}
				if cfg.BcryptCost != 10 {
					t.Errorf("BcryptCost = %d, want %d", cfg.BcryptCost, 10)
				}
				if cfg.LoginRateLimitAttempts != 5 {
					t.Errorf("LoginRateLimitAttempts = %d, want %d", cfg.LoginRateLimitAttempts, 5)
				}
				if cfg.LoginRateLimitWindow != 15*time.Minute {
					t.Errorf("LoginRateLimitWindow = %v, want %v", cfg.LoginRateLimitWindow, 15*time.Minute)
				}
			},
		},
		{
			name: "custom values override defaults",
			envVars: map[string]string{
				"DATABASE_URL":           "postgres://localhost/db",
				"HYDRA_ADMIN_URL":        "http://localhost:4445",
				"GRPC_PORT":              "50052",
				"HTTP_PORT":              "8052",
				"BCRYPT_COST":            "12",
				"LOGIN_RATE_LIMIT_ATTEMPTS": "10",
				"LOGIN_RATE_LIMIT_WINDOW":   "30m",
			},
			wantErr: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				if cfg.GRPCPort != 50052 {
					t.Errorf("GRPCPort = %d, want %d", cfg.GRPCPort, 50052)
				}
				if cfg.HTTPPort != 8052 {
					t.Errorf("HTTPPort = %d, want %d", cfg.HTTPPort, 8052)
				}
				if cfg.BcryptCost != 12 {
					t.Errorf("BcryptCost = %d, want %d", cfg.BcryptCost, 12)
				}
				if cfg.LoginRateLimitAttempts != 10 {
					t.Errorf("LoginRateLimitAttempts = %d, want %d", cfg.LoginRateLimitAttempts, 10)
				}
				if cfg.LoginRateLimitWindow != 30*time.Minute {
					t.Errorf("LoginRateLimitWindow = %v, want %v", cfg.LoginRateLimitWindow, 30*time.Minute)
				}
			},
		},
		{
			name: "fails when DATABASE_URL is missing",
			envVars: map[string]string{
				"HYDRA_ADMIN_URL": "http://localhost:4445",
			},
			wantErr: true,
		},
		{
			name: "fails when HYDRA_ADMIN_URL is missing",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://localhost/db",
			},
			wantErr: true,
		},
		{
			name: "fails when bcrypt cost is too low",
			envVars: map[string]string{
				"DATABASE_URL":    "postgres://localhost/db",
				"HYDRA_ADMIN_URL": "http://localhost:4445",
				"BCRYPT_COST":     "3",
			},
			wantErr: true,
		},
		{
			name: "fails when bcrypt cost is too high",
			envVars: map[string]string{
				"DATABASE_URL":    "postgres://localhost/db",
				"HYDRA_ADMIN_URL": "http://localhost:4445",
				"BCRYPT_COST":     "32",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars
			os.Clearenv()

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkConfig != nil {
				tt.checkConfig(t, cfg)
			}
		})
	}
}

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	"github.com/daisuke8000/example-ec-platform/bff/internal/authz"
	"github.com/daisuke8000/example-ec-platform/bff/internal/client"
	"github.com/daisuke8000/example-ec-platform/bff/internal/config"
	"github.com/daisuke8000/example-ec-platform/bff/internal/handler"
	"github.com/daisuke8000/example-ec-platform/bff/internal/jwt"
	"github.com/daisuke8000/example-ec-platform/bff/internal/middleware"
	"github.com/daisuke8000/example-ec-platform/bff/internal/observability"
	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"

	"go.opentelemetry.io/otel/metric"
)

type Dependencies struct {
	Config        *config.Config
	JWKSManager   *jwt.JWKSManager
	Validator     *jwt.Validator
	RateLimiter   *middleware.RateLimiter
	PublicMatcher *middleware.PublicEndpointMatcher
	Metrics       *observability.AuthMetrics

	// Backend service clients
	UserServiceClient userv1connect.UserServiceClient

	// Authorization
	Authorizer *authz.Authorizer

	// Handlers
	UserHandler *handler.UserServiceProxy
}

func NewDependencies(ctx context.Context, cfg *config.Config, meter metric.Meter) (*Dependencies, error) {
	if cfg.JWT.IssuerURL == "" || cfg.JWT.Audience == "" {
		return nil, errors.New("missing required JWT configuration")
	}
	if cfg.JWKS.URL == "" {
		return nil, errors.New("missing required JWKS URL")
	}

	jwksManager, err := jwt.NewJWKSManager(ctx, jwt.JWKSConfig{
		URL:                cfg.JWKS.URL,
		RefreshInterval:    cfg.JWKS.RefreshInterval,
		MinRefreshInterval: cfg.JWKS.MinRefreshInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JWKS manager: %w", err)
	}

	validator := jwt.NewValidator(jwt.ValidatorConfig{
		Issuer:    cfg.JWT.IssuerURL,
		Audience:  cfg.JWT.Audience,
		ClockSkew: cfg.JWT.ClockSkew,
	}, jwksManager)

	rateLimiter := middleware.NewRateLimiter(middleware.RateLimitConfig{
		FailureThreshold: cfg.RateLimit.FailureThreshold,
		Window:           cfg.RateLimit.Window,
		Cooldown:         cfg.RateLimit.Cooldown,
	})

	var success bool
	defer func() {
		if !success {
			jwksManager.Close()
			rateLimiter.Close()
		}
	}()

	publicMatcher := middleware.NewPublicEndpointMatcher(cfg.GetPublicEndpoints())

	var metrics *observability.AuthMetrics
	if meter != nil {
		metrics, err = observability.NewAuthMetrics(meter)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		metrics.SetDependencyStatus("hydra", jwksManager.IsHealthy())
	}

	// Initialize backend service clients
	userServiceClient := client.NewUserServiceClient(client.UserClientConfig{
		BaseURL: cfg.Backend.UserServiceURL,
		Timeout: cfg.Backend.RequestTimeout,
	})

	// Initialize authorization
	authorizer := authz.NewAuthorizer()

	// Initialize handlers
	logger := slog.Default()
	userHandler := handler.NewUserServiceProxy(userServiceClient, authorizer, logger)

	success = true
	return &Dependencies{
		Config:            cfg,
		JWKSManager:       jwksManager,
		Validator:         validator,
		RateLimiter:       rateLimiter,
		PublicMatcher:     publicMatcher,
		Metrics:           metrics,
		UserServiceClient: userServiceClient,
		Authorizer:        authorizer,
		UserHandler:       userHandler,
	}, nil
}

func (d *Dependencies) Close() {
	if d.RateLimiter != nil {
		d.RateLimiter.Close()
	}
	if d.JWKSManager != nil {
		d.JWKSManager.Close()
	}
}

func BuildInterceptorChain(deps *Dependencies) connect.Option {
	authInterceptor := middleware.NewAuthInterceptor(
		middleware.AuthInterceptorConfig{
			TrustedProxyHeader: deps.Config.Server.TrustedProxyHeader,
		},
		deps.Validator,
		deps.RateLimiter,
		deps.PublicMatcher,
	)

	return connect.WithInterceptors(authInterceptor)
}

func BuildHTTPHandler(cfg *config.Config, connectHandler http.Handler) http.Handler {
	sanitizer := middleware.NewHeaderSanitizer(cfg.HeadersToSanitize())

	if connectHandler == nil {
		connectHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	}

	return sanitizer.Middleware(connectHandler)
}

// RegisterHandlers registers all Connect-go service handlers to the mux.
func (d *Dependencies) RegisterHandlers(mux *http.ServeMux) {
	interceptors := BuildInterceptorChain(d)

	// Register User Service handler
	path, handler := userv1connect.NewUserServiceHandler(d.UserHandler, interceptors)
	mux.Handle(path, handler)
}

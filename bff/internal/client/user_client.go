package client

import (
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"
	pkgmw "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
)

type UserClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

func NewUserServiceClient(cfg UserClientConfig) userv1connect.UserServiceClient {
	httpClient := NewH2CClient(cfg.Timeout)
	return newUserServiceClientWithHTTP(httpClient, cfg.BaseURL)
}

func newUserServiceClientWithHTTP(httpClient *http.Client, baseURL string) userv1connect.UserServiceClient {
	return userv1connect.NewUserServiceClient(
		httpClient,
		baseURL,
		connect.WithInterceptors(
			pkgmw.ClientPropagatorInterceptor(),
		),
	)
}

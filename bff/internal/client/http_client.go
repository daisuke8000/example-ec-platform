package client

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// NewH2CClient creates an HTTP client configured for h2c (HTTP/2 over cleartext).
// This is used for internal service-to-service communication.
func NewH2CClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
}

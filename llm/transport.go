package llm

import (
	"net/http"
	"time"
)

// newPooledHTTPClient returns an http.Client with connection pooling and
// keep-alive enabled. Reusing connections saves ~100-200ms on TLS handshake
// for subsequent requests to the same provider.
func newPooledHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     120 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
}

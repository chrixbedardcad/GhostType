package llm

import (
	"net/http"
	"time"
)

// newPooledHTTPClient returns an http.Client with connection pooling and
// keep-alive enabled. Reusing connections saves ~100-200ms on TLS handshake
// for subsequent requests to the same provider.
// If timeoutMs > 0, it is used as the client timeout in milliseconds;
// otherwise a default of 120s is used.
func newPooledHTTPClient(timeoutMs int) *http.Client {
	timeout := 120 * time.Second
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     120 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
}

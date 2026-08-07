package provider

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// HTTPDebugEnabled reports whether verbose HTTP request/response logging is on.
// Enable it by starting Foghorn with FOGHORN_HTTP_DEBUG=1 (also accepts
// true/yes/on). Credentials are never logged — the Authorization header and
// userinfo in URLs are redacted.
func HTTPDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FOGHORN_HTTP_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// withHTTPDebug wraps inner so every request/response is logged when debug is
// enabled. When disabled it returns inner unchanged; passing nil leaves the
// http.Client on its default transport.
func withHTTPDebug(inner http.RoundTripper) http.RoundTripper {
	if !HTTPDebugEnabled() {
		return inner
	}
	if inner == nil {
		inner = http.DefaultTransport
	}
	return debugTransport{inner: inner}
}

type debugTransport struct {
	inner http.RoundTripper
}

func (d debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := d.inner.RoundTrip(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		log.Printf("http: %s %s -> ERROR after %s: %v", req.Method, req.URL.Redacted(), elapsed, err)
		return resp, err
	}
	line := fmt.Sprintf("http: %s %s -> %d (%s)", req.Method, req.URL.Redacted(), resp.StatusCode, elapsed)
	// A Location header on a 3xx is the smoking gun for a proxy rewriting the
	// request path/scheme — log it so redirect downgrades are visible.
	if loc := resp.Header.Get("Location"); loc != "" {
		line += " Location=" + loc
	}
	log.Print(line)
	return resp, err
}

package guardrails

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RestrictedClient returns an *http.Client whose transport refuses any request
// to a host:port not derived from the given backend URLs. Connectors built
// with this client physically cannot phone home.
func RestrictedClient(backendURLs []string) *http.Client {
	allowed := map[string]bool{}
	for _, raw := range backendURLs {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue // unparsable entries grant nothing
		}
		allowed[u.Host] = true
	}
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: &allowlistTransport{allowed: allowed, next: http.DefaultTransport},
	}
}

type allowlistTransport struct {
	allowed map[string]bool
	next    http.RoundTripper
}

func (t *allowlistTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.allowed[req.URL.Host] {
		return nil, fmt.Errorf("request to %q blocked: host is not in the configured backend allowlist", req.URL.Host)
	}
	return t.next.RoundTrip(req)
}

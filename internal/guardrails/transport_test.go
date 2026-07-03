package guardrails

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRestrictedClientAllowsConfiguredBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	client := RestrictedClient([]string{srv.URL})
	resp, err := client.Get(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("Get(allowed host) = %v", err)
	}
	resp.Body.Close()
}

func TestRestrictedClientBlocksUnknownHost(t *testing.T) {
	allowed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(allowed.Close)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request reached a host outside the allowlist")
	}))
	t.Cleanup(other.Close)

	client := RestrictedClient([]string{allowed.URL})
	_, err := client.Get(other.URL + "/exfiltrate")

	if err == nil {
		t.Fatal("Get(disallowed host) = nil error")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error %q does not mention the allowlist", err)
	}
}

func TestRestrictedClientIgnoresUnparsableAllowlistEntries(t *testing.T) {
	client := RestrictedClient([]string{"://bad"})
	if _, err := client.Get("http://127.0.0.1:1/"); err == nil {
		t.Error("client with empty effective allowlist allowed a request")
	}
}

func TestRestrictedClientMatchesHostPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Same IP, different port must be blocked.
	client := RestrictedClient([]string{"http://" + u.Hostname() + ":1"})
	if _, err := client.Get(srv.URL); err == nil {
		t.Error("request to same host on non-allowlisted port succeeded")
	}
}

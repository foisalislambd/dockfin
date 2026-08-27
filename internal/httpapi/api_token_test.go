package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dockfin/dockfin/internal/config"
)

func TestRateLimitIPCloudflare(t *testing.T) {
	a := &API{Cfg: &config.Config{TrustProxy: true}}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("CF-Connecting-IP", "203.0.113.88")
	if got := a.rateLimitIP(r); got != "203.0.113.88" {
		t.Fatalf("got %q", got)
	}
	a.Cfg.TrustProxy = false
	if got := a.rateLimitIP(r); got == "203.0.113.88" {
		t.Fatalf("must not trust CF header without TrustProxy, got %q", got)
	}
}

func TestAPITokenAllows(t *testing.T) {
	if !apiTokenAllows([]string{"read"}, http.MethodGet, "/api/v1/applications") {
		t.Fatal("read should allow GET apps")
	}
	if apiTokenAllows([]string{"read"}, http.MethodPost, "/api/v1/applications/x/deploy") {
		t.Fatal("read should deny deploy")
	}
	if !apiTokenAllows([]string{"deploy"}, http.MethodPost, "/api/v1/applications/x/deploy") {
		t.Fatal("deploy should allow deploy")
	}
	if apiTokenAllows([]string{"read"}, http.MethodGet, "/api/v1/env-vars") {
		t.Fatal("read should deny sensitive GET")
	}
	if !apiTokenAllows([]string{"read:sensitive"}, http.MethodGet, "/api/v1/env-vars") {
		t.Fatal("read:sensitive should allow env-vars")
	}
	if !apiTokenAllows([]string{"write"}, http.MethodDelete, "/api/v1/services/x") {
		t.Fatal("write should allow delete")
	}
	if !apiTokenAllows([]string{"read"}, http.MethodPost, "/api/v1/mcp") {
		t.Fatal("read should allow MCP POST (JSON-RPC read tools)")
	}
	if !apiTokenAllows([]string{"deploy"}, http.MethodPost, "/api/v1/mcp") {
		t.Fatal("deploy should allow MCP POST")
	}
	if apiTokenAllows([]string{}, http.MethodPost, "/api/v1/mcp") {
		t.Fatal("empty abilities should deny MCP")
	}
}

func TestClientIPAllowed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.10:9999"
	if !clientIPAllowed(r, "") {
		t.Fatal("empty allowlist should allow")
	}
	if !clientIPAllowed(r, "203.0.113.10") {
		t.Fatal("exact IP should allow")
	}
	if clientIPAllowed(r, "10.0.0.1") {
		t.Fatal("other IP should deny")
	}
	if !clientIPAllowed(r, "203.0.113.0/24") {
		t.Fatal("CIDR should allow")
	}
}

func TestCookieSessionWithoutBearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/deploy", nil)
	r.AddCookie(&http.Cookie{Name: "dockfin_session", Value: "sess"})
	if !cookieSessionWithoutBearer(r) {
		t.Fatal("expected cookie-only session")
	}
	r.Header.Set("Authorization", "Bearer tok")
	if cookieSessionWithoutBearer(r) {
		t.Fatal("bearer should disable cookie-only flag")
	}
}

func TestSessionTokenPrefersBearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "dockfin_session", Value: "cookie-token"})
	r.Header.Set("Authorization", "Bearer api-token")
	if got := sessionToken(r); got != "api-token" {
		t.Fatalf("expected bearer to win, got %q", got)
	}
}

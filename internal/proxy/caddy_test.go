package proxy

import (
	"strings"
	"testing"
)

func TestTraefikLabelsHTTPSRedirectIndependentOfTLS(t *testing.T) {
	tls := TraefikLabelsHTTPS("web", "app.example.com", "80", false)
	joined := strings.Join(tls, "\n")
	if !strings.Contains(joined, "certresolver=letsencrypt") {
		t.Fatalf("custom domain must keep TLS when redirect is off:\n%s", joined)
	}
	if strings.Contains(joined, "redirectscheme") {
		t.Fatalf("did not expect HTTP bounce:\n%s", joined)
	}
	redir := TraefikLabelsHTTPS("web", "app.example.com", "80", true)
	if !strings.Contains(strings.Join(redir, "\n"), "redirectscheme") {
		t.Fatal("expected HTTP→HTTPS bounce when forceHTTPS")
	}
	magic := TraefikLabelsHTTPS("web", "app.1.2.3.4.sslip.io", "80", true)
	m := strings.Join(magic, "\n")
	if strings.Contains(m, "certresolver") || strings.Contains(m, "entrypoints=https") {
		t.Fatalf("magic domain must stay HTTP:\n%s", m)
	}
}

func TestCaddyLabels(t *testing.T) {
	httpOnly := CaddyLabels("app", "example.com", "80", false)
	if len(httpOnly) != 2 || httpOnly[0] != "caddy=http://example.com" {
		t.Fatalf("http-only labels: %#v", httpOnly)
	}
	if httpOnly[1] != "caddy.reverse_proxy={{upstreams 80}}" {
		t.Fatalf("upstream: %#v", httpOnly[1])
	}

	https := CaddyLabels("app", "example.com", "3000", true)
	if https[0] != "caddy=example.com" {
		t.Fatalf("https site: %#v", https[0])
	}

	// https:// prefix with forceHTTPS=false should still be HTTP-only
	coerced := CaddyLabels("app", "https://example.com", "80", false)
	if coerced[0] != "caddy=http://example.com" {
		t.Fatalf("coerce https→http: %#v", coerced[0])
	}

	if CaddyLabels("app", "", "80", true) != nil {
		t.Fatal("empty fqdn should yield nil")
	}
	if CaddyLabels("app", "bad host.com", "80", false) != nil {
		t.Fatal("space in host should yield nil")
	}
	if CaddyLabels("app", "example.com", "80x", false) != nil {
		t.Fatal("non-numeric port should yield nil")
	}
	if CaddyLabels("app", "ok.com", "8080", false) == nil {
		t.Fatal("valid labels expected")
	}
}

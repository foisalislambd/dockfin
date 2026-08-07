package proxy

import (
	"strings"
	"testing"
)

func TestSanitizePanelHost(t *testing.T) {
	if got := sanitizePanelHost("Me.Example.COM"); got != "me.example.com" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizePanelHost("evil`host"); got != "evilhost" {
		t.Fatalf("stripped backtick: %q", got)
	}
	if got := sanitizePanelHost("  app.example.com  "); got != "app.example.com" {
		t.Fatalf("trim: %q", got)
	}
}

func TestBuildPanelDynamicYAMLHTTPS(t *testing.T) {
	y := buildPanelDynamicYAML("me.mafizul.org", "http://dockfin:8000", true)
	for _, want := range []string{
		"Host(`me.mafizul.org`)",
		"entryPoints:",
		"- https",
		"certResolver: letsencrypt",
		"dockfin-panel-redirect",
		"url: http://dockfin:8000",
	} {
		if !strings.Contains(y, want) {
			t.Fatalf("missing %q in:\n%s", want, y)
		}
	}
}

func TestBuildPanelDynamicYAMLHTTPOnly(t *testing.T) {
	y := buildPanelDynamicYAML("app.example.com", "http://dockfin:8000", false)
	if strings.Contains(y, "https") || strings.Contains(y, "letsencrypt") {
		t.Fatalf("http-only must not enable TLS:\n%s", y)
	}
	if !strings.Contains(y, "- http") {
		t.Fatalf("expected http entrypoint:\n%s", y)
	}
}

func TestWantAutoHTTPSForPanelDomain(t *testing.T) {
	if !WantAutoHTTPS("https://me.mafizul.org") {
		t.Fatal("custom domain should want HTTPS")
	}
	if WantAutoHTTPS("http://x.1.2.3.4.sslip.io") {
		t.Fatal("magic must not want HTTPS")
	}
}

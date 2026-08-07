package httpapi

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/dockfin/dockfin/internal/config"
)

func TestPublicBaseURLPriority(t *testing.T) {
	r := &http.Request{Host: "req.example:8000", Header: http.Header{}}

	t.Run("cfg when no store", func(t *testing.T) {
		a := &API{Cfg: &config.Config{PublicURL: "http://178.18.243.148:8000"}}
		got := a.publicBaseURL(r)
		if got != "http://178.18.243.148:8000" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("request host when no cfg", func(t *testing.T) {
		a := &API{}
		got := a.publicBaseURL(r)
		if got != "http://req.example:8000" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("https via forwarded proto", func(t *testing.T) {
		a := &API{}
		rf := &http.Request{
			RemoteAddr: "127.0.0.1:443",
			Host:       "panel.example.com",
			Header:     http.Header{"X-Forwarded-Proto": []string{"https"}, "X-Forwarded-Host": []string{"panel.example.com"}},
		}
		got := a.publicBaseURL(rf)
		if got != "https://panel.example.com" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("trims trailing slash", func(t *testing.T) {
		a := &API{Cfg: &config.Config{PublicURL: "https://dash.example.com/"}}
		got := a.publicBaseURL(r)
		if got != "https://dash.example.com" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestCookieSecureForRequest(t *testing.T) {
	cfg := &config.Config{CookieSecure: false}
	httpsReq := &http.Request{
		TLS:    &tls.ConnectionState{},
		Header: http.Header{"X-Forwarded-Proto": []string{"http"}},
	}
	if !cookieSecureForRequest(httpsReq, cfg) {
		t.Fatal("expected secure when request TLS is set")
	}
	spoofed := &http.Request{
		RemoteAddr: "127.0.0.1:1234",
		Header:     http.Header{"X-Forwarded-Proto": []string{"https"}},
	}
	if cookieSecureForRequest(spoofed, cfg) {
		t.Fatal("must not trust spoofable X-Forwarded-Proto for Secure cookies")
	}
	cfg.CookieSecure = true
	plain := &http.Request{Header: http.Header{}}
	if !cookieSecureForRequest(plain, cfg) {
		t.Fatal("expected cfg CookieSecure fallback")
	}
}

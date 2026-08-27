package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoveryURL(t *testing.T) {
	got := DiscoveryURL("https://idp.example.com/realms/app/")
	want := "https://idp.example.com/realms/app/.well-known/openid-configuration"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	already := "https://idp.example.com/.well-known/openid-configuration"
	if DiscoveryURL(already) != already {
		t.Fatalf("should keep existing well-known URL")
	}
}

func TestValidateIssuerURL(t *testing.T) {
	if err := ValidateIssuerURL("https://login.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIssuerURL("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIssuerURL("http://idp.example.com"); err == nil {
		t.Fatal("expected https requirement")
	}
	if err := ValidateIssuerURL("not-a-url"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "http://127.0.0.1",
			"authorization_endpoint": "http://127.0.0.1/auth",
			"token_endpoint":         "http://127.0.0.1/token",
			"userinfo_endpoint":      "http://127.0.0.1/userinfo",
		})
	}))
	defer srv.Close()

	doc, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(doc.AuthorizationEndpoint, "/auth") {
		t.Fatalf("auth endpoint: %s", doc.AuthorizationEndpoint)
	}

	if _, err := Fetch(context.Background(), srv.URL+"/missing"); err == nil {
		t.Fatal("expected missing document error")
	}
}

func TestDocumentValidate(t *testing.T) {
	d := &Document{AuthorizationEndpoint: "a", TokenEndpoint: "t"}
	if err := d.Validate(); err == nil {
		t.Fatal("expected missing userinfo")
	}
}

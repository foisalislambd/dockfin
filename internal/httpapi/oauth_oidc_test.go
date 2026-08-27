package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func TestOIDCUserinfoMissingEmailVerified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"abc","email":"u@example.com","name":"U"}`))
	}))
	defer srv.Close()
	conf := &oauth2.Config{}
	tok := &oauth2.Token{AccessToken: "tok", TokenType: "Bearer"}
	p, err := fetchOauthProfile(context.Background(), conf, tok, srv.URL, "oidc")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "abc" || p.Email != "u@example.com" {
		t.Fatalf("%+v", p)
	}
	if !p.EmailVerified {
		t.Fatal("OIDC should treat missing email_verified as true when email is present")
	}
}

func TestAuthentikUserinfoRequiresEmailVerifiedClaim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"abc","email":"u@example.com"}`))
	}))
	defer srv.Close()
	p, err := fetchOauthProfile(context.Background(), &oauth2.Config{}, &oauth2.Token{AccessToken: "t"}, srv.URL, "authentik")
	if err != nil {
		t.Fatal(err)
	}
	if p.EmailVerified {
		t.Fatal("authentik without email_verified must stay false")
	}
}

func TestOauthPKCEExchangeOptions(t *testing.T) {
	opts, err := oauthPKCEExchangeOptions("github", "leftover-verifier")
	if err != nil || opts != nil {
		t.Fatalf("github must ignore PKCE: opts=%v err=%v", opts, err)
	}
	if _, err := oauthPKCEExchangeOptions("oidc", ""); err == nil {
		t.Fatal("oidc requires verifier")
	}
	opts, err = oauthPKCEExchangeOptions("oidc", "abc")
	if err != nil || len(opts) != 1 {
		t.Fatalf("oidc PKCE: opts=%v err=%v", opts, err)
	}
}

func TestOauthStateCookieBindsProvider(t *testing.T) {
	if oauthStateCookieValue("oidc", "abc") == oauthStateCookieValue("github", "abc") {
		t.Fatal("state must be provider-scoped")
	}
}

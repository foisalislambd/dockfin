package secrets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeMetaAndSettings(t *testing.T) {
	if _, err := NormalizeMeta("doppler", Metadata{TimeoutSeconds: 31, BaseURL: "http://evil", Namespace: "a\nb"}); err == nil {
		t.Fatal("expected timeout error before provider switch")
	}
	doppler, err := NormalizeMeta(" Doppler ", Metadata{BaseURL: "http://evil", ClientID: "x", Namespace: "n", TimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	if doppler.BaseURL != "" || doppler.ClientID != "" || doppler.Namespace != "" {
		t.Fatalf("doppler meta not cleared: %+v", doppler)
	}
	if _, err := NormalizeMeta("vault", Metadata{Namespace: "ok\rbad", BaseURL: "https://vault.example"}); err == nil {
		t.Fatal("expected newline namespace")
	}
	if _, err := NormalizeMeta("infisical", Metadata{BaseURL: "http://127.0.0.1:8080"}); err == nil || !strings.Contains(err.Error(), "client id") {
		t.Fatalf("client id: %v", err)
	}
	if _, err := NormalizeMeta("vault", Metadata{}); err == nil {
		t.Fatal("expected vault address")
	}
	if _, err := NormalizeMeta("aws", Metadata{}); err == nil {
		t.Fatal("expected unsupported")
	}

	if _, err := NormalizeSettings("infisical", Settings{ProjectID: "p", Environment: "prod", SecretPath: "no-slash"}); err == nil {
		t.Fatal("expected slash")
	}
	if _, err := NormalizeSettings("infisical", Settings{ProjectID: "p", Environment: "prod", SecretPath: "/../x"}); err == nil {
		t.Fatal("expected dot-dot")
	}
	inf, err := NormalizeSettings("infisical", Settings{ProjectID: "p", Environment: "prod", Project: "leftover"})
	if err != nil || inf.SecretPath != "/" || inf.Project != "" {
		t.Fatalf("%+v %v", inf, err)
	}
	if _, err := NormalizeSettings("vault", Settings{Mount: "secret", Path: "app/../../root"}); err == nil {
		t.Fatal("expected vault traversal")
	}
	if _, err := NormalizeSettings("vault", Settings{Mount: "secret", Path: "app?x=1"}); err == nil {
		t.Fatal("expected query in path")
	}
	vault, err := NormalizeSettings("VAULT", Settings{Path: "/app/prod/"})
	if err != nil || vault.Mount != "secret" || vault.Path != "app/prod" {
		t.Fatalf("%+v %v", vault, err)
	}
	if _, err := NormalizeSettings("nope", Settings{}); err == nil {
		t.Fatal("expected unsupported settings")
	}
}

func TestValidateBaseURLEdges(t *testing.T) {
	got, err := ValidateBaseURL("http://127.0.0.1:8200/v1/")
	if err != nil || got != "http://127.0.0.1:8200/v1" {
		t.Fatalf("%q %v", got, err)
	}
	for _, raw := range []string{
		"",
		"vault.example",
		"http://user:pass@127.0.0.1:8200",
		"http://0.0.0.0/",
		"http://[fe80::1]/",
		"https://metadata.google.com/path",
		"http://127.0.0.1/#frag",
	} {
		_, err := ValidateBaseURL(raw)
		if raw == "http://127.0.0.1/#frag" {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	stripped, err := ValidateBaseURL("http://127.0.0.1:8200/v1/?q=1#x")
	if err != nil || strings.Contains(stripped, "q=1") || strings.Contains(stripped, "#") {
		t.Fatalf("%q %v", stripped, err)
	}
}

func TestFetchEncodesAndRejects(t *testing.T) {
	var project, config, secretPath string
	var redirects int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/secrets/download"):
			project = r.URL.Query().Get("project")
			config = r.URL.Query().Get("config")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TOKEN":"v"}`))
		case strings.HasSuffix(r.URL.Path, "/login"):
			http.Error(w, `{"message":"nope secret=csecret"}`, http.StatusUnauthorized)
		case strings.Contains(r.URL.Path, "/api/v4/secrets"):
			secretPath = r.URL.Query().Get("secretPath")
			_, _ = w.Write([]byte(`{"secrets":[]}`))
		default:
			redirects++
			http.Redirect(w, r, "http://169.254.169.254/latest", http.StatusFound)
		}
	}))
	t.Cleanup(srv.Close)
	prev := dopplerAPI
	dopplerAPI = srv.URL
	t.Cleanup(func() { dopplerAPI = prev })

	ctx := context.Background()
	got, err := Fetch(ctx, "doppler", "tok", Metadata{}, Settings{Project: "app&x=1", Config: "prd y"})
	if err != nil {
		t.Fatal(err)
	}
	if got["TOKEN"] != "v" || project != "app&x=1" || config != "prd y" {
		t.Fatalf("project=%q config=%q got=%#v", project, config, got)
	}
	if _, err := Fetch(ctx, "doppler", "  ", Metadata{}, Settings{}); err == nil {
		t.Fatal("expected token")
	}
	if _, err := Fetch(ctx, "custom", "tok", Metadata{}, Settings{}); err == nil {
		t.Fatal("expected unsupported provider")
	}
	if err := Validate(ctx, "custom", "tok", Metadata{}); err == nil {
		t.Fatal("expected unsupported validate")
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	t.Cleanup(bad.Close)
	dopplerAPI = bad.URL
	if _, err := Fetch(ctx, "doppler", "tok", Metadata{}, Settings{}); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("%v", err)
	}

	dopplerAPI = srv.URL
	_, err = Fetch(ctx, "infisical", "csecret-should-not-leak", Metadata{BaseURL: srv.URL, ClientID: "cid"}, Settings{ProjectID: "p", Environment: "prod"})
	if err == nil || strings.Contains(err.Error(), "csecret-should-not-leak") {
		t.Fatalf("login error leaked or missing: %v", err)
	}
	if _, err := Fetch(ctx, "vault", "tok", Metadata{BaseURL: srv.URL}, Settings{Path: "../secret"}); err == nil {
		t.Fatal("expected traversal")
	}
	if _, err := Fetch(ctx, "vault", "tok", Metadata{BaseURL: "http://169.254.169.254"}, Settings{Path: "app"}); err == nil {
		t.Fatal("expected metadata base")
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/login") {
			_, _ = w.Write([]byte(`{"accessToken":"access"}`))
			return
		}
		secretPath = r.URL.Query().Get("secretPath")
		if r.URL.Query().Get("environment") != "prod&hack=1" {
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"secrets":[{"secretKey":"K","secretValue":"V"}]}`))
	}))
	t.Cleanup(ok.Close)
	vals, err := Fetch(ctx, "infisical", "csecret", Metadata{BaseURL: ok.URL, ClientID: "cid"}, Settings{ProjectID: "p", Environment: "prod&hack=1", SecretPath: "/app"})
	if err != nil {
		t.Fatal(err)
	}
	if vals["K"] != "V" || secretPath != "/app" {
		t.Fatalf("vals=%#v path=%q", vals, secretPath)
	}
	if redirects != 0 {
		t.Fatalf("unexpected redirect handler hits %d", redirects)
	}
}

func TestVaultPayloadAndRedirectNotFollowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/secret/data/app/prod" {
			t.Errorf("path %s", r.URL.Path)
		}
		if strings.Contains(r.URL.RawPath, "..") || strings.Contains(r.URL.Path, " ") {
			t.Errorf("unescaped path %s", r.URL.RequestURI())
		}
		http.Redirect(w, r, "http://169.254.169.254/latest", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	_, err := Fetch(context.Background(), "vault", "tok", Metadata{BaseURL: srv.URL, Namespace: "admin"}, Settings{Mount: "secret", Path: "app/prod"})
	if err == nil || strings.Contains(err.Error(), "169.254") {
		t.Fatalf("redirect followed or missing error: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(bad.Close)
	if _, err := Fetch(context.Background(), "vault", "tok", Metadata{BaseURL: bad.URL}, Settings{Path: "app"}); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("%v", err)
	}
}

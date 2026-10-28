package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchProviders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer doppler-token" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"workplace":"ok"}`))
	})
	mux.HandleFunc("/v3/configs/config/secrets/download", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"DB_PASSWORD": "s3cret", "COUNT": 2})
	})
	mux.HandleFunc("/api/v1/auth/universal-auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["clientId"] != "cid" || body["clientSecret"] != "csecret" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "access"})
	})
	mux.HandleFunc("/api/v4/secrets", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"imports": []any{map[string]any{"secrets": []any{map[string]string{"secretKey": "FROM_IMPORT", "secretValue": "a"}}}},
			"secrets": []any{map[string]string{"secretKey": "FROM_IMPORT", "secretValue": "override"}, map[string]string{"secretKey": "OWN", "secretValue": "b"}},
		})
	})
	mux.HandleFunc("/v1/auth/token/lookup-self", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "vault-token" || r.Header.Get("X-Vault-Namespace") != "admin" {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/v1/secret/data/app/prod", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": map[string]any{"API_KEY": "k"}}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	prev := dopplerAPI
	dopplerAPI = srv.URL
	t.Cleanup(func() { dopplerAPI = prev })

	ctx := context.Background()
	doppler, err := Fetch(ctx, "doppler", "doppler-token", Metadata{}, Settings{Project: "app", Config: "prd"})
	if err != nil {
		t.Fatal(err)
	}
	if doppler["DB_PASSWORD"] != "s3cret" || doppler["COUNT"] != "2" {
		t.Fatalf("doppler: %#v", doppler)
	}
	if err := Validate(ctx, "doppler", "doppler-token", Metadata{}); err != nil {
		t.Fatal(err)
	}
	inf, err := Fetch(ctx, "infisical", "csecret", Metadata{BaseURL: srv.URL, ClientID: "cid"}, Settings{ProjectID: "proj", Environment: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if inf["FROM_IMPORT"] != "override" || inf["OWN"] != "b" {
		t.Fatalf("infisical: %#v", inf)
	}
	vault, err := Fetch(ctx, "vault", "vault-token", Metadata{BaseURL: srv.URL, Namespace: "admin"}, Settings{Mount: "secret", Path: "app/prod"})
	if err != nil {
		t.Fatal(err)
	}
	if vault["API_KEY"] != "k" {
		t.Fatalf("vault: %#v", vault)
	}
	if err := Validate(ctx, "infisical", "csecret", Metadata{BaseURL: srv.URL, ClientID: "cid"}); err != nil {
		t.Fatal(err)
	}
	if err := Validate(ctx, "vault", "vault-token", Metadata{BaseURL: srv.URL, Namespace: "admin"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBaseURLBlocksMetadata(t *testing.T) {
	if _, err := ValidateBaseURL("http://169.254.169.254/latest"); err == nil {
		t.Fatal("expected metadata IP to be rejected")
	}
	if _, err := ValidateBaseURL("https://metadata.google.internal"); err == nil {
		t.Fatal("expected metadata host to be rejected")
	}
	if _, err := ValidateBaseURL("ftp://vault.example"); err == nil {
		t.Fatal("expected scheme to be rejected")
	}
}

func TestProviderErrorOmitsBodyDump(t *testing.T) {
	msg := providerError(403, []byte(`{"errors":["permission denied"],"secret":"should-not-be-the-whole-body"}`))
	if !strings.Contains(msg, "permission denied") || strings.Contains(msg, "should-not") {
		t.Fatalf("msg: %s", msg)
	}
}

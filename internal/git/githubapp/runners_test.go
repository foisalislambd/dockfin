package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateJITConfig(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/9/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case "/orgs/acme/actions/runners/generate-jitconfig":
			if r.Header.Get("Authorization") != "Bearer installation-token" {
				t.Errorf("auth %s", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"runner":             map[string]any{"id": 42},
				"encoded_jit_config": "secret-config",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	app := &App{
		AppID:         "1",
		PrivateKeyPEM: string(pemKey),
		APIURL:        srv.URL,
		Organization:  "acme",
		HTTP:          srv.Client(),
	}
	got, err := app.GenerateJITConfig("9", "dockfin-runner", 7, []string{"self-hosted", "linux", "dockfin"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 42 || got.EncodedConfig != "secret-config" {
		t.Fatalf("%+v", got)
	}
}

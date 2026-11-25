package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/testkit"
)

func TestGitSourceAndServerCRUD(t *testing.T) {
	t.Chdir(t.TempDir())
	st := testkit.OpenStore(t)
	api := &API{
		Cfg: &config.Config{
			Env: "development", PublicURL: "http://127.0.0.1:8000",
			MasterKey: "test-master-key-for-integration-32b!", SessionTTL: time.Hour,
		},
		Store:  st,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(api.Router())
	t.Cleanup(srv.Close)
	token := registerOwner(t, srv.Client(), srv.URL)
	do := func(method, path string, body any) (int, []byte) {
		t.Helper()
		var rdr io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, srv.URL+path, rdr)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, raw
	}
	code, raw := do(http.MethodPost, "/api/v1/private-keys", map[string]string{
		"name": "edge", "private_key": "not-a-real-key",
	})
	if code != http.StatusCreated && code != http.StatusOK && code != http.StatusBadRequest {
		t.Fatalf("key %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/servers", map[string]any{
		"name": "edge", "ip": "10.8.8.8", "port": 22, "user_name": "root", "proxy_type": "caddy",
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("server %d %s", code, raw)
	}
	var created map[string]any
	_ = json.Unmarshal(raw, &created)
	serverID, _ := created["id"].(string)
	if serverID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodGet, "/api/v1/servers/"+serverID, nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte("10.8.8.8")) {
		t.Fatalf("get server %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/git-sources", map[string]any{
		"name": "acme", "provider": "github", "html_url": "https://github.com", "api_url": "https://api.github.com",
		"custom_user": "git", "custom_port": 22,
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("source %d %s", code, raw)
	}
	_ = json.Unmarshal(raw, &created)
	sourceID, _ := created["id"].(string)
	if sourceID == "" {
		if nested, ok := created["source"].(map[string]any); ok {
			sourceID, _ = nested["id"].(string)
		}
	}
	if sourceID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodPatch, "/api/v1/git-sources/"+sourceID, map[string]any{
		"name": "acme-2", "organization": "acme",
	})
	if code != http.StatusOK || !bytes.Contains(raw, []byte("acme-2")) {
		t.Fatalf("patch source %d %s", code, raw)
	}
	code, raw = do(http.MethodGet, "/api/v1/git-sources", nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte("acme-2")) {
		t.Fatalf("list %d %s", code, raw)
	}
	code, raw = do(http.MethodDelete, "/api/v1/git-sources/"+sourceID, nil)
	if code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("delete %d %s", code, raw)
	}
}

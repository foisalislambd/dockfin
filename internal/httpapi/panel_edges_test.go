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

func TestTagsPasskeysAndHookValidation(t *testing.T) {
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
	team := mustTeam(t, st, "owner@example.com")
	_, env, err := st.CreateProject(t.Context(), team, "panel", "")
	if err != nil {
		t.Fatal(err)
	}
	app := mustApp(t, st, team, env.ID, "web")
	do := func(method, path string, body any) (int, []byte) {
		t.Helper()
		var rdr io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
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
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, raw
	}

	code, raw := do(http.MethodPost, "/api/v1/tags", map[string]any{"name": "  ", "color": "#000"})
	if code < 400 {
		t.Fatalf("blank tag %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/tags", map[string]any{"name": "prod'; drop table tags;--", "color": "red"})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("tag %d %s", code, raw)
	}
	var tag map[string]any
	if err := json.Unmarshal(raw, &tag); err != nil {
		t.Fatal(err)
	}
	tagID, _ := tag["id"].(string)
	if tagID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodPost, "/api/v1/tags/attach", map[string]any{
		"tag_id": tagID, "resource_type": "application", "resource_id": app.ID.String(),
	})
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusNoContent {
		t.Fatalf("attach %d %s", code, raw)
	}
	code, raw = do(http.MethodGet, "/api/v1/tags/"+tagID+"/resources", nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte(app.ID.String())) && !bytes.Contains(raw, []byte("application")) {
		t.Fatalf("tag resources %d %s", code, raw)
	}
	code, raw = do(http.MethodDelete, "/api/v1/tags/"+tagID+"/attach?resource_type=application&resource_id="+app.ID.String(), nil)
	if code >= 500 {
		t.Fatalf("detach %d %s", code, raw)
	}

	code, raw = do(http.MethodGet, "/api/v1/settings/passkeys", nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte("enabled")) {
		t.Fatalf("passkeys %d %s", code, raw)
	}
	code, raw = do(http.MethodPut, "/api/v1/settings/passkeys", map[string]any{"enabled": true})
	if code != http.StatusOK || !bytes.Contains(raw, []byte("true")) {
		t.Fatalf("enable passkeys %d %s", code, raw)
	}
	code, raw = do(http.MethodGet, "/api/v1/auth/passkeys", nil)
	if code != http.StatusOK {
		t.Fatalf("list passkeys %d %s", code, raw)
	}

	code, raw = do(http.MethodPost, "/api/v1/gitlab/system-hooks", map[string]any{
		"gitlab_url": "http://169.254.169.254", "token": "pat",
	})
	if code != http.StatusBadRequest || !bytes.Contains(raw, []byte("https")) {
		t.Fatalf("gitlab url %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/gitlab/system-hooks", map[string]any{
		"gitlab_url": "https://gitlab.com", "token": "   ",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("gitlab token %d %s", code, raw)
	}
	code, raw = do(http.MethodGet, "/api/v1/gitlab/system-hooks", nil)
	if code != http.StatusOK {
		t.Fatalf("list hooks %d %s", code, raw)
	}
	code, raw = do(http.MethodGet, "/api/v1/fleet/metrics", nil)
	if code >= 500 {
		t.Fatalf("fleet %d %s", code, raw)
	}
}

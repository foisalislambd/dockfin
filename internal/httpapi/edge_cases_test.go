package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/testkit"
)

func TestHandlerEdgesRejectAbuseAndPersistTheRest(t *testing.T) {
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
	_, env, err := st.CreateProject(t.Context(), team, "edges", "")
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

	code, raw := do(http.MethodPost, "/api/v1/services", map[string]any{
		"environment_id": env.ID.String(), "name": "cache", "fqdn": "javascript:alert(1)",
		"docker_compose_raw": "services:\n  redis:\n    image: redis:7\n",
	})
	if code != http.StatusBadRequest || !bytes.Contains(raw, []byte("fqdn")) {
		t.Fatalf("bad fqdn %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/services", map[string]any{
		"environment_id": "not-a-uuid", "name": "cache",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("bad env %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/services", map[string]any{
		"environment_id": env.ID.String(), "name": "cache", "template": "not-a-real-template",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("template %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/services", map[string]any{
		"environment_id": env.ID.String(), "name": "cache",
		"docker_compose_raw": "services:\n  redis:\n    image: redis:7\n",
		"fqdn":               "cache.example.com",
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("service %d %s", code, raw)
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	svcID, _ := created["id"].(string)
	if svcID == "" {
		if nested, ok := created["service"].(map[string]any); ok {
			svcID, _ = nested["id"].(string)
		}
	}
	if svcID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodGet, "/api/v1/services/"+svcID, nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte("cache.example.com")) && !bytes.Contains(raw, []byte("redis")) {
		t.Fatalf("get service %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/services/"+svcID+"/deploy", map[string]any{})
	if code == http.StatusOK || code == http.StatusAccepted || code == http.StatusCreated {
		if !bytes.Contains(bytes.ToLower(raw), []byte("destination")) && !bytes.Contains(bytes.ToLower(raw), []byte("server")) {
			t.Fatalf("deploy without a server succeeded: %d %s", code, raw)
		}
	}

	code, raw = do(http.MethodPost, "/api/v1/s3-storages", map[string]any{
		"name": "bad", "endpoint": "http://169.254.169.254/latest", "bucket": "b",
		"access_key": "ak", "secret_key": "sk",
	})
	if code != http.StatusBadRequest || !bytes.Contains(bytes.ToLower(raw), []byte("endpoint")) {
		t.Fatalf("metadata s3 %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/s3-storages", map[string]any{
		"name": "minio", "endpoint": "http://127.0.0.1:9000", "bucket": "backups",
		"access_key": "ak", "secret_key": "sk", "path_style": true,
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("s3 %d %s", code, raw)
	}
	if bytes.Contains(raw, []byte("sk")) && bytes.Contains(raw, []byte("secret_key")) {
		t.Fatalf("secret echoed %s", raw)
	}
	var storage map[string]any
	_ = json.Unmarshal(raw, &storage)
	storageID, _ := storage["id"].(string)
	if storageID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodGet, "/api/v1/s3-storages/"+storageID, nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte("backups")) {
		t.Fatalf("get s3 %d %s", code, raw)
	}

	code, raw = do(http.MethodPost, "/api/v1/scheduled-backups", map[string]any{
		"resource_type": "volume", "resource_id": app.ID.String(), "frequency": "0 3 * * *",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("backup type %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/scheduled-backups", map[string]any{
		"resource_type": "application", "resource_id": app.ID.String(),
		"frequency": "0 3 * * *", "retention": 3, "s3_storage_id": storageID,
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("backup %d %s", code, raw)
	}
	var backup map[string]any
	_ = json.Unmarshal(raw, &backup)
	backupID, _ := backup["id"].(string)
	if backupID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodPatch, "/api/v1/scheduled-backups/"+backupID, map[string]any{
		"retention": 1, "enabled": false,
	})
	if code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("patch backup %d %s", code, raw)
	}

	code, raw = do(http.MethodPost, "/api/v1/scheduled-tasks", map[string]any{
		"resource_id": app.ID.String(), "name": "nightly", "frequency": "0 4 * * *",
		"command": "echo ok; rm -rf /", "container_name": "bad name",
	})
	if code != http.StatusBadRequest || !bytes.Contains(raw, []byte("container")) {
		t.Fatalf("task container %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/scheduled-tasks", map[string]any{
		"resource_type": "application", "resource_id": app.ID.String(),
		"name": "nightly", "command": "echo ok", "frequency": "0 4 * * *",
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("task %d %s", code, raw)
	}
	if !bytes.Contains(raw, []byte("echo ok")) {
		t.Fatalf("command not stored %s", raw)
	}
	var task map[string]any
	_ = json.Unmarshal(raw, &task)
	taskID, _ := task["id"].(string)
	if taskID == "" {
		t.Fatal(string(raw))
	}
	code, raw = do(http.MethodGet, "/api/v1/scheduled-tasks/"+taskID+"/executions", nil)
	if code != http.StatusOK {
		t.Fatalf("executions %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/scheduled-tasks/"+taskID+"/run", map[string]any{})
	if code != http.StatusServiceUnavailable || !bytes.Contains(raw, []byte("scheduler")) {
		t.Fatalf("run task %d %s", code, raw)
	}

	code, raw = do(http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": app.ID.String(),
		"key": "PORT\nEVIL", "value": "80",
	})
	if code != http.StatusBadRequest || !bytes.Contains(raw, []byte("key")) {
		t.Fatalf("env key %d %s", code, raw)
	}
	code, raw = do(http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": app.ID.String(),
		"key": "PORT", "value": "80",
	})
	if code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("env %d %s", code, raw)
	}
	code, raw = do(http.MethodGet, "/api/v1/env-vars?resource_type=application&resource_id="+app.ID.String(), nil)
	if code != http.StatusOK || !bytes.Contains(raw, []byte("PORT")) || bytes.Contains(raw, []byte("EVIL")) {
		t.Fatalf("list env %d %s", code, raw)
	}

	code, raw = do(http.MethodGet, "/api/v1/applications/"+app.ID.String()+"/previews", nil)
	if code != http.StatusOK {
		t.Fatalf("previews %d %s", code, raw)
	}
	code, raw = do(http.MethodDelete, "/api/v1/s3-storages/"+storageID, nil)
	if code != http.StatusOK && code != http.StatusNoContent && code != http.StatusConflict {
		t.Fatalf("delete s3 %d %s", code, raw)
	}
	if strings.Contains(string(raw), "secret_key") {
		t.Fatalf("secret on delete %s", raw)
	}
}

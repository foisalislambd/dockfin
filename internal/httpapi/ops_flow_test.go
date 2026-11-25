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
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
)

func TestServiceBackupAndTaskFlow(t *testing.T) {
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
	_, env, err := st.CreateProject(t.Context(), team, "ops", "")
	if err != nil {
		t.Fatal(err)
	}
	app := mustApp(t, st, team, env.ID, "web")
	db, err := st.CreateDatabase(t.Context(), &store.Database{
		TeamID: team, EnvironmentID: env.ID, Name: "pg", Engine: "postgresql",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path string, body any) (int, map[string]any) {
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
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		if parsed == nil {
			parsed = map[string]any{"_raw": string(raw)}
		}
		return res.StatusCode, parsed
	}
	code, templates := do(http.MethodGet, "/api/v1/services/templates", nil)
	if code != http.StatusOK {
		t.Fatalf("templates %d %v", code, templates)
	}
	code, created := do(http.MethodPost, "/api/v1/services", map[string]any{
		"environment_id": env.ID.String(), "name": "redis",
		"docker_compose_raw": "services:\n  redis:\n    image: redis:7\n",
		"fqdn":               "redis.example.com",
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("service %d %v", code, created)
	}
	svcID, _ := created["id"].(string)
	if svcID == "" {
		if nested, ok := created["service"].(map[string]any); ok {
			svcID, _ = nested["id"].(string)
		}
	}
	if svcID == "" {
		t.Fatal(created)
	}
	code, patched := do(http.MethodPatch, "/api/v1/services/"+svcID, map[string]any{
		"name": "redis2", "description": "cache", "is_force_https": true,
	})
	patchedRaw, _ := json.Marshal(patched)
	if code != http.StatusOK || !bytes.Contains(patchedRaw, []byte("redis2")) {
		t.Fatalf("patch %d %v", code, patched)
	}
	code, s3 := do(http.MethodPost, "/api/v1/s3-storages", map[string]any{
		"name": "minio", "endpoint": "http://127.0.0.1:9000", "bucket": "backups",
		"region": "us-east-1", "access_key": "ak", "secret_key": "sk", "path_style": true,
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("s3 %d %v", code, s3)
	}
	for _, item := range []struct{ kind, id string }{
		{"database", db.ID.String()},
		{"application", app.ID.String()},
		{"service", svcID},
	} {
		code, backup := do(http.MethodPost, "/api/v1/scheduled-backups", map[string]any{
			"resource_type": item.kind, "resource_id": item.id, "frequency": "0 3 * * *", "retention": 3,
		})
		if code != http.StatusCreated && code != http.StatusOK {
			t.Fatalf("backup %s %d %v", item.kind, code, backup)
		}
	}
	code, listed := do(http.MethodGet, "/api/v1/scheduled-backups?resource_type=database&resource_id="+db.ID.String(), nil)
	if code != http.StatusOK {
		t.Fatalf("list backups %d %v", code, listed)
	}
	code, task := do(http.MethodPost, "/api/v1/scheduled-tasks", map[string]any{
		"resource_type": "application", "resource_id": app.ID.String(),
		"name": "nightly", "command": "echo ok", "frequency": "0 4 * * *",
	})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("task %d %v", code, task)
	}
	code, tasks := do(http.MethodGet, "/api/v1/scheduled-tasks?resource_type=application&resource_id="+app.ID.String(), nil)
	if code != http.StatusOK {
		t.Fatalf("tasks %d %v", code, tasks)
	}
	code, teamTasks := do(http.MethodGet, "/api/v1/scheduled-task-executions", nil)
	if code != http.StatusOK && code != http.StatusNotFound {
		t.Fatalf("executions %d %v", code, teamTasks)
	}
}

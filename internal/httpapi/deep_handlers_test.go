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
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
)

func TestHandlersExerciseMutations(t *testing.T) {
	t.Chdir(t.TempDir())
	st := testkit.OpenStore(t)
	api := &API{
		Cfg: &config.Config{
			Env: "development", PublicURL: "http://127.0.0.1:8000",
			MasterKey: "test-master-key-for-integration-32b!", SessionTTL: time.Hour,
			RateLimitPerMinute: 0, AuthRateLimitPer15Min: 0, WebhookRateLimitPerMinute: 0,
		},
		Store:  st,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(api.Router())
	t.Cleanup(srv.Close)
	token := registerOwner(t, srv.Client(), srv.URL)
	do := func(method, path string, body any) (int, map[string]any) {
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
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		return res.StatusCode, parsed
	}
	code, created := do(http.MethodPost, "/api/v1/projects", map[string]string{"name": "deep", "description": "d"})
	if code != http.StatusCreated {
		t.Fatalf("project %d %v", code, created)
	}
	envID := created["environment"].(map[string]any)["id"].(string)
	projectID := created["project"].(map[string]any)["id"].(string)
	code, app := do(http.MethodPost, "/api/v1/applications", map[string]string{
		"environment_id": envID, "name": "web", "build_pack": "dockerfile", "dockerfile": "FROM nginx:alpine\n",
	})
	if code != http.StatusCreated {
		t.Fatalf("app %d %v", code, app)
	}
	appID, _ := app["id"].(string)
	if appID == "" {
		appID = app["application"].(map[string]any)["id"].(string)
	}
	code, patched := do(http.MethodPatch, "/api/v1/applications/"+appID, map[string]any{
		"name": "web2", "description": "hello", "fqdn": "web.example.com", "git_branch": "main",
		"git_repository": "https://github.com/acme/web.git", "ports_exposes": "80", "ports_mappings": "80:80",
		"install_command": "true", "build_command": "true", "start_command": "nginx", "base_directory": "/",
		"dockerfile_location": "/Dockerfile", "compose_prepare": true, "is_force_https": true,
		"is_preview_enabled": true, "is_auto_deploy_enabled": true, "watch_paths": "apps/**",
		"health_check_enabled": true, "health_check_path": "/health", "health_check_port": 80,
		"health_check_method": "GET", "health_check_return_code": 200, "health_check_scheme": "http",
		"limits_cpus": "1", "limits_memory": "256m", "is_spa": true, "is_gzip_enabled": true,
		"custom_docker_restart_policy": "unless-stopped", "docker_images_to_keep": 2,
	})
	if code != http.StatusOK {
		t.Fatalf("patch app %d %v", code, patched)
	}
	if patched["name"] != "web2" && (patched["application"] == nil || patched["application"].(map[string]any)["name"] != "web2") {
		if !strings.Contains(mustJSON(patched), "web2") {
			t.Fatalf("patch did not rename: %v", patched)
		}
	}
	code, db := do(http.MethodPost, "/api/v1/databases", map[string]any{
		"environment_id": envID, "name": "pg", "engine": "postgresql", "password": "secret-pass",
	})
	if code != http.StatusCreated {
		t.Fatalf("db %d %v", code, db)
	}
	code, svc := do(http.MethodPost, "/api/v1/services", map[string]string{
		"environment_id": envID, "name": "n8n", "service_type": "n8n",
		"docker_compose_raw": "services:\n  n8n:\n    image: n8nio/n8n:latest\n",
	})
	if code != http.StatusCreated {
		t.Fatalf("service %d %v", code, svc)
	}
	code, ev := do(http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": appID, "key": "PORT", "value": "80", "is_runtime": true,
	})
	if code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("env %d %v", code, ev)
	}
	code, listed := do(http.MethodGet, "/api/v1/env-vars?resource_type=application&resource_id="+appID, nil)
	if code != http.StatusOK || !strings.Contains(mustJSON(listed), "PORT") {
		t.Fatalf("list env %d %v", code, listed)
	}
	code, s3 := do(http.MethodPost, "/api/v1/s3-storages", map[string]any{
		"name": "b2", "endpoint": "https://s3.example.com", "bucket": "bucket", "region": "us-east-1",
		"access_key": "ak", "secret_key": "sk",
	})
	if code != http.StatusCreated {
		t.Fatalf("s3 %d %v", code, s3)
	}
	if strings.Contains(mustJSON(s3), "sk") && strings.Contains(mustJSON(s3), "secret_key") {
		t.Fatal("s3 response echoed the secret key")
	}
	code, tag := do(http.MethodPost, "/api/v1/tags", map[string]string{"name": "prod", "color": "#112233"})
	if code != http.StatusCreated {
		t.Fatalf("tag %d %v", code, tag)
	}
	code, key := do(http.MethodPost, "/api/v1/private-keys/generate", map[string]string{"name": "ci", "type": "ed25519"})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("key %d %v", code, key)
	}
	if strings.Contains(mustJSON(key), "PRIVATE KEY") {
		t.Fatal("generated private key was returned in the API response")
	}
	code, inv := do(http.MethodPost, "/api/v1/team/invitations", map[string]string{"email": "new@example.com", "role": "member"})
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("invite %d %v", code, inv)
	}
	code, ov := do(http.MethodGet, "/api/v1/overview", nil)
	if code != http.StatusOK || !strings.Contains(mustJSON(ov), "web2") && !strings.Contains(mustJSON(ov), projectID) {
		t.Fatalf("overview %d %v", code, ov)
	}
	code, settings := do(http.MethodGet, "/api/v1/settings", nil)
	if code != http.StatusOK {
		t.Fatalf("settings %d %v", code, settings)
	}
	_ = store.Application{}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

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

func TestUpdateApplicationPersistsTheForm(t *testing.T) {
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
	_, env, err := st.CreateProject(t.Context(), team, "upd", "")
	if err != nil {
		t.Fatal(err)
	}
	app := mustApp(t, st, team, env.ID, "web")
	body := map[string]any{
		"name": "web-renamed", "description": "hello", "fqdn": "web.example.com",
		"build_pack": "dockerfile", "git_repository": "https://github.com/acme/web.git", "git_branch": "main",
		"ports_exposes": "80", "ports_mappings": "8080:80", "base_directory": "/",
		"dockerfile": "FROM nginx:alpine\n", "dockerfile_location": "/Dockerfile",
		"install_command": "true", "build_command": "true", "start_command": "nginx -g 'daemon off;'",
		"publish_directory": "/usr/share/nginx/html", "watch_paths": "src/**",
		"is_force_https": true, "redirect_permanent": false, "is_preview_enabled": true,
		"is_auto_deploy_enabled": true, "is_git_submodules_enabled": false, "is_preserve_repository_enabled": true,
		"health_check_enabled": true, "health_check_path": "/", "health_check_port": 80,
		"health_check_method": "GET", "health_check_return_code": 200, "health_check_interval": 5,
		"health_check_timeout": 5, "health_check_retries": 3, "health_check_host": "localhost",
		"health_check_scheme": "http", "health_check_response_text": "ok", "health_check_start_period": 1,
		"health_check_type": "http", "limits_memory": "256m", "limits_cpus": "0.5",
		"redirect": "both", "is_disable_build_cache": false, "is_git_shallow_clone_enabled": true,
		"is_git_lfs_enabled": false, "is_gpu_enabled": true, "gpu_count": 1, "gpu_driver": "nvidia",
		"gpu_device_ids": "GPU-0", "custom_docker_stop_timeout": 10, "custom_docker_restart_policy": "unless-stopped",
		"custom_docker_max_restart_count": 3, "is_spa": false, "is_gzip_enabled": true,
		"is_stripprefix_enabled": false, "is_log_drain_enabled": false, "is_debug_enabled": false,
		"is_env_sorting_enabled": true, "is_pr_deployments_public_enabled": false, "skip_rebuild_if_unchanged": true,
		"is_consistent_container_name_enabled": true, "custom_internal_name": "web",
		"docker_images_to_keep": 2, "logs_line_limit": 100, "swarm_replicas": 1,
		"swarm_placement_constraints": "node.role==worker", "is_swarm_only_worker_nodes": false,
		"is_include_timestamps": true, "inject_build_args_to_dockerfile": true, "use_build_secrets": false,
		"include_source_commit_in_build": true, "custom_labels": "dockfin=1",
		"custom_network_aliases": "web", "preview_url_template": "{{pr_id}}.preview.example.com",
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/applications/"+app.ID.String(), bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	resp, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update %d %s", res.StatusCode, resp)
	}
	if !bytes.Contains(resp, []byte("web-renamed")) {
		t.Fatalf("name not saved: %s", resp)
	}
	saved, err := st.GetApplication(t.Context(), team, app.ID)
	if err != nil || saved.Name != "web-renamed" || saved.FQDN == "" || !saved.HealthCheckEnabled || !saved.IsGPUEnabled {
		t.Fatalf("%+v %v", saved, err)
	}
}

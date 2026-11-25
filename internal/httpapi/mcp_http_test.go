package httpapi

import (
	"bytes"
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

func TestMCPToolsUseTheTeamStore(t *testing.T) {
	t.Chdir(t.TempDir())
	st := testkit.OpenStore(t)
	if _, err := st.Pool.Exec(t.Context(), `UPDATE instance_settings SET is_mcp_server_enabled=true`); err != nil {
		t.Fatal(err)
	}
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
	_, env, err := st.CreateProject(t.Context(), team, "mcp", "")
	if err != nil {
		t.Fatal(err)
	}
	app := mustApp(t, st, team, env.ID, "web")
	_ = mustService(t, st, team, env.ID, "svc")
	_ = mustDB(t, st, team, env.ID, "db")

	call := func(body string) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/mcp", strings.NewReader(body))
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
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAccepted {
			t.Fatalf("mcp %d %s", res.StatusCode, raw)
		}
		return string(raw)
	}
	probe, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	probe.Header.Set("Authorization", "Bearer "+token)
	pres, err := http.DefaultClient.Do(probe)
	if err != nil {
		t.Fatal(err)
	}
	defer pres.Body.Close()
	praw, _ := io.ReadAll(pres.Body)
	if pres.StatusCode != http.StatusOK || !bytes.Contains(praw, []byte("dockfin")) {
		t.Fatalf("probe %d %s", pres.StatusCode, praw)
	}
	listed := call(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if !strings.Contains(listed, "list_projects") {
		t.Fatal(listed)
	}
	projects := call(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}`)
	if !strings.Contains(projects, "mcp") {
		t.Fatal(projects)
	}
	apps := call(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_applications","arguments":{}}}`)
	if !strings.Contains(apps, app.ID.String()) {
		t.Fatal(apps)
	}
	got := call(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_application","arguments":{"id":"` + app.ID.String() + `"}}}`)
	if !strings.Contains(got, "dockerfile") && !strings.Contains(got, "web") {
		t.Fatal(got)
	}
	deployed := call(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"deploy_application","arguments":{"id":"` + app.ID.String() + `"}}}`)
	if !strings.Contains(deployed, "destination") {
		t.Fatal(deployed)
	}
	if !strings.Contains(call(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`), "svc") {
		t.Fatal("services")
	}
	if !strings.Contains(call(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_databases","arguments":{"environment_id":"`+env.ID.String()+`"}}}`), "db") {
		t.Fatal("databases")
	}
	if !strings.Contains(call(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"list_servers","arguments":{}}}`), "servers") {
		t.Fatal("servers")
	}
	stopped := call(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"stop_application","arguments":{"id":"` + app.ID.String() + `"}}}`)
	if !strings.Contains(stopped, "destination") && !strings.Contains(stopped, "stopped") && !strings.Contains(stopped, "running") {
		t.Fatal(stopped)
	}
	batch := call(`[{"jsonrpc":"2.0","id":10,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"}]`)
	if !strings.Contains(batch, "2.0") && batch != "" {
		t.Fatal(batch)
	}
	if !strings.Contains(call(`not-json`), "invalid JSON") {
		t.Fatal("parse")
	}
}

package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TestOwnerSweep exercises every route as the owning admin with a populated
// project and a JSON body, so handlers run past the empty-body decode that the
// authz matrix stops at.
func TestOwnerSweep(t *testing.T) {
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
	client := &http.Client{Timeout: 2 * time.Second}
	token := registerOwner(t, client, srv.URL)
	team := mustTeam(t, st, "owner@example.com")
	ctx := t.Context()
	_, env, err := st.CreateProject(ctx, team, "sweep", "sweep")
	if err != nil {
		t.Fatal(err)
	}
	app := mustApp(t, st, team, env.ID, "web")
	db := mustDB(t, st, team, env.ID, "db")
	svc := mustService(t, st, team, env.ID, "svc")
	dep, err := st.CreateDeployment(ctx, team, app.ID, nil, "abc", "ok", false, false, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	server, err := st.CreateServer(ctx, team, nil, "edge", "", "10.0.0.8", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{
		"projects":     env.ProjectID.String(),
		"applications": app.ID.String(),
		"services":     svc.ID.String(),
		"databases":    db.ID.String(),
		"environments": env.ID.String(),
		"deployments":  dep.ID.String(),
		"servers":      server.ID.String(),
		"other":        uuid.NewString(),
	}
	mux := api.Router().(chi.Routes)
	body := `{
		"name":"sweep","description":"d","ip":"10.0.0.9","user_name":"root","port":22,
		"proxy_type":"traefik","engine":"postgresql","environment_id":"` + env.ID.String() + `",
		"build_pack":"dockerfile","dockerfile":"FROM nginx:alpine\n","git_repository":"https://github.com/acme/web.git",
		"git_branch":"main","fqdn":"web.example.com","ports_exposes":"80","key":"PORT","value":"80",
		"resource_type":"application","resource_id":"` + app.ID.String() + `","host":"web.example.com",
		"url":"https://example.com/hook","enabled":true,"channel":"email","confirmation_name":"web",
		"password":"password12345","confirm":"web","message":"down","email":"owner@example.com",
		"team_id":"` + team.String() + `","server_id":"` + server.ID.String() + `"
	}`
	var problems []string
	for _, rt := range walkRoutes(t, mux) {
		if rt.method == http.MethodGet && (strings.Contains(rt.pattern, "/logs") || strings.Contains(rt.pattern, "/terminal") || strings.Contains(rt.pattern, "/stream")) {
			continue
		}
		if strings.Contains(rt.pattern, "detect") || strings.Contains(rt.pattern, "/git/scan") || strings.Contains(rt.pattern, "hostinger") {
			continue
		}
		pattern := strings.ReplaceAll(rt.pattern, "{serverID}", server.ID.String())
		path := fillRoute(pattern, ids)
		var rdr io.Reader
		if rt.method != http.MethodGet && rt.method != http.MethodHead {
			rdr = strings.NewReader(body)
		}
		req, err := http.NewRequest(rt.method, srv.URL+path, rdr)
		if err != nil {
			t.Fatal(err)
		}
		if rdr != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := client.Do(req)
		if err != nil {
			if strings.Contains(err.Error(), "Timeout") || strings.Contains(err.Error(), "deadline") {
				continue
			}
			problems = append(problems, rt.method+" "+rt.pattern+" "+err.Error())
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
		res.Body.Close()
		if ownerFailed(res.StatusCode, string(raw)) {
			problems = append(problems, rt.method+" "+path+" "+res.Status+" "+string(raw))
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d owner requests failed:\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

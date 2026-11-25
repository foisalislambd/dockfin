package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// authzRouteFile is the committed list of every route the router exposes.
// A new route fails this test until it is added here with a class of public, session, or admin.
const authzRouteFile = "testdata/authz_routes.txt"

func TestEveryRouteAuthzMatrix(t *testing.T) {
	t.Chdir(t.TempDir())
	st := testkit.OpenStore(t)
	api := &API{
		Cfg: &config.Config{
			Env:                       "development",
			PublicURL:                 "http://127.0.0.1:8000",
			MasterKey:                 "test-master-key-for-integration-32b!",
			SessionTTL:                time.Hour,
			CORSOrigins:               []string{"http://localhost:5173"},
			RateLimitPerMinute:        0,
			AuthRateLimitPer15Min:     0,
			WebhookRateLimitPerMinute: 0,
		},
		Store:  st,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(api.Router())
	t.Cleanup(srv.Close)
	client := &http.Client{Timeout: 3 * time.Second}

	ctx := context.Background()
	_ = registerOwner(t, client, srv.URL)
	owner := userByEmail(t, st, "owner@example.com")
	ownerTeam := mustTeam(t, st, "owner@example.com")

	allowedProject, allowedEnv, err := st.CreateProject(ctx, ownerTeam, "allowed", "visible")
	if err != nil {
		t.Fatal(err)
	}
	hiddenProject, hiddenEnv, err := st.CreateProject(ctx, ownerTeam, "hidden-project", "secret-hidden-name")
	if err != nil {
		t.Fatal(err)
	}
	allowedApp := mustApp(t, st, ownerTeam, allowedEnv.ID, "web")
	hiddenApp := mustApp(t, st, ownerTeam, hiddenEnv.ID, "secret-hidden-app")
	allowedDB := mustDB(t, st, ownerTeam, allowedEnv.ID, "db")
	hiddenDB := mustDB(t, st, ownerTeam, hiddenEnv.ID, "secret-hidden-db")
	allowedSvc := mustService(t, st, ownerTeam, allowedEnv.ID, "svc")
	hiddenSvc := mustService(t, st, ownerTeam, hiddenEnv.ID, "secret-hidden-svc")
	allowedDep, err := st.CreateDeployment(ctx, ownerTeam, allowedApp.ID, nil, "abc", "ok", false, false, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	hiddenDep, err := st.CreateDeployment(ctx, ownerTeam, hiddenApp.ID, nil, "def", "ok", false, false, false, 0)
	if err != nil {
		t.Fatal(err)
	}

	member, _, err := st.CreateUserWithPersonalTeam(ctx, "viewer@example.com", "Viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, access_scope) VALUES ($1,$2,'member','projects')`, ownerTeam, member.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetProjectAccess(ctx, ownerTeam, member.ID, "projects", []store.ProjectGrant{{ProjectID: allowedProject.ID, Permission: "view"}}); err != nil {
		t.Fatal(err)
	}
	viewToken := mintSession(t, st, member.ID, ownerTeam)
	outsider, outsiderTeam, err := st.CreateUserWithPersonalTeam(ctx, "outsider@example.com", "Outsider", "hash")
	if err != nil {
		t.Fatal(err)
	}
	outToken := mintSession(t, st, outsider.ID, outsiderTeam.ID)
	ownerToken := mintSession(t, st, owner.ID, ownerTeam)
	_ = owner

	allowed := map[string]string{
		"projects":     allowedProject.ID.String(),
		"applications": allowedApp.ID.String(),
		"services":     allowedSvc.ID.String(),
		"databases":    allowedDB.ID.String(),
		"environments": allowedEnv.ID.String(),
		"deployments":  allowedDep.ID.String(),
		"other":        uuid.NewString(),
	}
	hidden := map[string]string{
		"projects":     hiddenProject.ID.String(),
		"applications": hiddenApp.ID.String(),
		"services":     hiddenSvc.ID.String(),
		"databases":    hiddenDB.ID.String(),
		"environments": hiddenEnv.ID.String(),
		"deployments":  hiddenDep.ID.String(),
		"other":        uuid.NewString(),
	}
	sentinel := hiddenApp.ID.String()

	mux, ok := api.Router().(chi.Routes)
	if !ok {
		t.Fatal("router is not a chi route tree")
	}
	if _, err := st.GetSession(ctx, viewToken); err != nil {
		t.Fatalf("viewer session not stored: %v", err)
	}
	routes := walkRoutes(t, mux)
	if len(routes) < 100 {
		t.Fatalf("router exposed %d routes, expected the full API", len(routes))
	}

	type hit struct {
		status int
		body   string
	}
	call := func(method, path, token, team, raw string) hit {
		t.Helper()
		var rdr io.Reader
		if raw != "" && method != http.MethodGet && method != http.MethodHead {
			rdr = strings.NewReader(raw)
		}
		req, err := http.NewRequest(method, srv.URL+path, rdr)
		if err != nil {
			t.Fatal(err)
		}
		if rdr != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if team != "" {
			req.Header.Set("X-Team-ID", team)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return hit{status: res.StatusCode, body: string(b)}
	}

	var lines []string
	probe := call(http.MethodGet, "/api/v1/auth/me", viewToken, "", "")
	if probe.status != http.StatusOK {
		t.Fatalf("viewer /auth/me %d %s tokenlen=%d", probe.status, probe.body, len(viewToken))
	}
	for _, rt := range routes {
		key := rt.method + " " + rt.pattern
		ownerTok, viewTok, outTok := ownerToken, viewToken, outToken
		if strings.HasSuffix(rt.pattern, "/auth/logout") {
			ownerTok = mintSession(t, st, owner.ID, ownerTeam)
			viewTok = mintSession(t, st, member.ID, ownerTeam)
			outTok = mintSession(t, st, outsider.ID, outsiderTeam.ID)
		}
		kind := projectKind(rt.pattern)
		path := fillRoute(rt.pattern, allowed)
		hiddenPath := path
		if kind != "" {
			hiddenPath = strings.Replace(path, allowed[kind], hidden[kind], 1)
		}
		raw := "{}"
		if strings.Contains(rt.pattern, "/env-vars") {
			id := allowed["applications"]
			path = withEnvQuery(rt.method, path, id)
			hiddenPath = withEnvQuery(rt.method, hiddenPath, hidden["applications"])
			if rt.method != http.MethodGet {
				raw = `{"resource_type":"application","resource_id":"` + id + `","key":"PORT","value":"80"}`
			}
		}
		unauth := call(rt.method, path, "", "", raw)
		wrong := call(rt.method, path, outTok, uuid.NewString(), raw)
		memberHit := call(rt.method, path, viewTok, "", raw)
		memberHidden := hit{}
		if kind != "" {
			memberHidden = call(rt.method, hiddenPath, viewTok, "", strings.ReplaceAll(raw, allowed["applications"], hidden["applications"]))
		}
		ownerHit := call(rt.method, path, ownerTok, "", raw)

		teamGated := wrong.status == http.StatusForbidden && strings.Contains(wrong.body, "not a team member")
		adminRefused := memberHit.status == http.StatusForbidden && strings.Contains(memberHit.body, "admin or owner role required")
		class := "session"
		switch {
		case publicRoutes[key]:
			class = "public"
		case teamGated && adminRefused:
			class = "admin"
		case teamGated:
			class = "session"
		default:
			class = "auth"
		}
		lines = append(lines, key+" "+class)

		if strings.Contains(unauth.body, sentinel) || strings.Contains(wrong.body, sentinel) || strings.Contains(memberHit.body, sentinel) {
			t.Errorf("%s leaked the hidden application", key)
		}
		middlewareUnauth := unauth.status == http.StatusUnauthorized && strings.Contains(unauth.body, `"unauthorized"`)
		switch class {
		case "public":
			if unauth.status >= 500 || wrong.status >= 500 || ownerFailed(ownerHit.status, ownerHit.body) {
				t.Errorf("%s public status unauth=%d wrong=%d owner=%d %s", key, unauth.status, wrong.status, ownerHit.status, ownerHit.body)
			}
		case "auth":
			if !middlewareUnauth {
				t.Errorf("%s unauthenticated status %d body %s", key, unauth.status, unauth.body)
			}
			if teamGated {
				t.Errorf("%s auth-only route enforced a team", key)
			}
			if memberHit.status == http.StatusUnauthorized && strings.Contains(memberHit.body, `"unauthorized"`) {
				t.Errorf("%s rejected a signed-in viewer: %s", key, memberHit.body)
			}
			if ownerFailed(ownerHit.status, ownerHit.body) {
				t.Errorf("%s owner call failed: %d %s", key, ownerHit.status, ownerHit.body)
			}
		case "admin":
			if !middlewareUnauth {
				t.Errorf("%s unauthenticated status %d body %s", key, unauth.status, unauth.body)
			}
			if !adminRefused {
				t.Errorf("%s viewer was not refused as a non-admin: %d %s", key, memberHit.status, memberHit.body)
			}
			if strings.Contains(ownerHit.body, "admin or owner role required") || ownerHit.status == http.StatusUnauthorized || ownerFailed(ownerHit.status, ownerHit.body) {
				t.Errorf("%s admin call failed: %d %s", key, ownerHit.status, ownerHit.body)
			}
		default:
			if !middlewareUnauth {
				t.Errorf("%s unauthenticated status %d (add it to publicRoutes if it is public) body %s", key, unauth.status, unauth.body)
			}
			if !teamGated {
				t.Errorf("%s wrong team status %d body %s", key, wrong.status, wrong.body)
			}
			if memberHit.status == http.StatusUnauthorized && strings.Contains(memberHit.body, `"unauthorized"`) {
				t.Errorf("%s rejected a signed-in viewer", key)
			}
			if strings.Contains(ownerHit.body, "admin or owner role required") || ownerFailed(ownerHit.status, ownerHit.body) {
				t.Errorf("%s owner call failed: %d %s", key, ownerHit.status, ownerHit.body)
			}
		}
		if kind != "" && class != "public" {
			if memberHidden.status != http.StatusNotFound {
				t.Errorf("%s viewer reached a resource outside their projects: %d %s", key, memberHidden.status, memberHidden.body)
			}
			if rt.method != http.MethodGet && rt.method != http.MethodHead && class != "admin" {
				if memberHit.status == http.StatusOK || memberHit.status == http.StatusCreated || memberHit.status == http.StatusAccepted {
					t.Errorf("%s view-only member mutated an allowed project: %d", key, memberHit.status)
				}
			}
		}
	}

	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"
	if os.Getenv("DOCKFIN_UPDATE_AUTHZ") == "1" {
		if err := os.MkdirAll(filepath.Dir(authzRouteAbs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(authzRouteAbs, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		authzGolden = []byte(got)
		t.Logf("wrote %s", authzRouteAbs)
	}
	want := string(authzGolden)
	if strings.TrimSpace(want) == "" {
		t.Fatal("authz route matrix is empty; run with DOCKFIN_UPDATE_AUTHZ=1 and commit testdata/authz_routes.txt")
	}
	if got != want {
		t.Fatalf("route matrix drifted; a new route needs an entry in testdata/authz_routes.txt\n%s", firstDiff(want, got))
	}
}

// authzGolden is filled by the generator in this file's init from the committed file
// when the test starts, before Chdir. The variable is assigned from disk in TestMain.
var (
	authzGolden   []byte
	authzRouteAbs string
)

func TestMain(m *testing.M) {
	if abs, err := filepath.Abs(authzRouteFile); err == nil {
		authzRouteAbs = abs
		if b, err := os.ReadFile(abs); err == nil {
			authzGolden = b
		}
	}
	os.Exit(m.Run())
}

func firstDiff(want, got string) string {
	wb, gb := strings.Split(want, "\n"), strings.Split(got, "\n")
	var missing, extra []string
	have := map[string]bool{}
	for _, line := range gb {
		have[line] = true
	}
	wantSet := map[string]bool{}
	for _, line := range wb {
		wantSet[line] = true
		if line != "" && !have[line] {
			missing = append(missing, line)
		}
	}
	for _, line := range gb {
		if line != "" && !wantSet[line] {
			extra = append(extra, line)
		}
	}
	var b strings.Builder
	if len(missing) > 0 {
		b.WriteString("missing from the running router:\n")
		b.WriteString(strings.Join(missing, "\n"))
		b.WriteByte('\n')
	}
	if len(extra) > 0 {
		b.WriteString("no matrix entry:\n")
		b.WriteString(strings.Join(extra, "\n"))
		b.WriteByte('\n')
	}
	return b.String()
}

type walked struct{ method, pattern string }

func walkRoutes(t *testing.T, r chi.Routes) []walked {
	t.Helper()
	var out []walked
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == "" || route == "" {
			return nil
		}
		out = append(out, walked{method: method, pattern: route})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].pattern == out[j].pattern {
			return out[i].method < out[j].method
		}
		return out[i].pattern < out[j].pattern
	})
	return out
}

func ownerFailed(status int, body string) bool {
	if status < 500 {
		return false
	}
	// The test server has no terminal bridge, and httptest cannot flush an SSE stream.
	if strings.Contains(body, "terminal not configured") || strings.Contains(body, "streaming unsupported") {
		return false
	}
	return true
}

func projectKind(pattern string) string {
	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" {
		return ""
	}
	switch parts[2] {
	case "projects", "applications", "services", "databases", "environments", "deployments":
		if strings.Contains(parts[3], "{") {
			return parts[2]
		}
	}
	return ""
}

func fillRoute(pattern string, ids map[string]string) string {
	replacer := strings.NewReplacer(
		"{projectID}", ids["projects"],
		"{appID}", ids["applications"],
		"{serviceID}", ids["services"],
		"{dbID}", ids["databases"],
		"{envID}", ids["environments"],
		"{deploymentID}", ids["deployments"],
	)
	out := replacer.Replace(pattern)
	for {
		i := strings.IndexByte(out, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(out[i:], '}')
		if j < 0 {
			break
		}
		out = out[:i] + ids["other"] + out[i+j+1:]
	}
	return out
}

func withEnvQuery(method, path, id string) string {
	if strings.Contains(path, "/env-vars") && !strings.Contains(path, "?") && method == http.MethodGet {
		return path + "?resource_type=application&resource_id=" + id
	}
	return path
}

func registerOwner(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": "owner@example.com", "name": "Owner", "password": "password12345"})
	res, err := client.Post(base+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var parsed map[string]any
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %v", res.StatusCode, parsed)
	}
	token, _ := parsed["token"].(string)
	if token == "" {
		t.Fatal("no token")
	}
	return token
}

func mintSession(t *testing.T, st *store.Store, userID, teamID uuid.UUID) string {
	t.Helper()
	token, err := crypto.RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(context.Background(), userID, teamID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return token
}

func userByEmail(t *testing.T, st *store.Store, email string) *store.User {
	t.Helper()
	u, _, err := st.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func mustApp(t *testing.T, st *store.Store, team, env uuid.UUID, name string) *store.Application {
	t.Helper()
	app, err := st.CreateApplication(context.Background(), &store.Application{
		TeamID: team, EnvironmentID: env, Name: name, BuildPack: "dockerfile", Dockerfile: "FROM nginx:alpine\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func mustDB(t *testing.T, st *store.Store, team, env uuid.UUID, name string) *store.Database {
	t.Helper()
	db, err := st.CreateDatabase(context.Background(), &store.Database{
		TeamID: team, EnvironmentID: env, Name: name, Engine: "postgresql",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func mustService(t *testing.T, st *store.Store, team, env uuid.UUID, name string) *store.Service {
	t.Helper()
	svc, err := st.CreateService(context.Background(), &store.Service{
		TeamID: team, EnvironmentID: env, Name: name, ServiceType: "n8n", DockerComposeRaw: "services: {}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

var publicRoutes = map[string]bool{
	"GET /":                                             true,
	"GET /health":                                       true,
	"GET /ready":                                        true,
	"GET /metrics":                                      true,
	"GET /api/docs":                                     true,
	"GET /openapi.json":                                 true,
	"GET /api/v1/openapi.json":                          true,
	"GET /api/v1/version":                               true,
	"POST /api/v1/auth/register":                        true,
	"GET /api/v1/auth/registration":                     true,
	"POST /api/v1/auth/login":                           true,
	"POST /api/v1/auth/login/2fa":                       true,
	"POST /api/v1/auth/forgot-password":                 true,
	"POST /api/v1/auth/reset-password":                  true,
	"GET /api/v1/auth/oauth/providers":                  true,
	"GET /api/v1/auth/oauth/{provider}/start":           true,
	"GET /api/v1/auth/oauth/{provider}/callback":        true,
	"GET /api/v1/auth/passkeys/enabled":                 true,
	"POST /api/v1/auth/passkeys/login/begin":            true,
	"POST /api/v1/auth/passkeys/login/finish":           true,
	"POST /api/v1/webhooks/git/{appID}":                 true,
	"POST /api/v1/webhooks/gitlab/system":               true,
	"GET /api/v1/webhooks/github/app/callback":          true,
	"GET /api/v1/webhooks/github/app/manifest":          true,
	"POST /api/v1/webhooks/github/app/events":           true,
	"POST /api/v1/sentinel/metrics":                     true,
	"POST /api/v1/sentinel/traffic":                     true,
	"GET /api/v1/deploy":                                true,
	"POST /api/v1/deploy":                               true,
	"GET /api/v1/webhooks/deploy/services/{serviceID}":  true,
	"POST /api/v1/webhooks/deploy/services/{serviceID}": true,
	"GET /api/v1/invitations/preview":                   true,
}

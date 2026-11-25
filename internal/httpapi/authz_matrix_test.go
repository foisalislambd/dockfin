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
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestAuthzAndEdgeMatrix(t *testing.T) {
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

	do := func(method, path, token, team string, body any) (*http.Response, map[string]any) {
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
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if team != "" {
			req.Header.Set("X-Team-ID", team)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		return res, parsed
	}
	status := func(method, path, token, team string, body any) int {
		t.Helper()
		res, _ := do(method, path, token, team, body)
		return res.StatusCode
	}

	res, parsed := do(http.MethodPost, "/api/v1/auth/register", "", "", map[string]string{
		"email": "owner@example.com", "name": "Owner", "password": "password12345",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %v", res.StatusCode, parsed)
	}
	ownerToken, _ := parsed["token"].(string)
	if ownerToken == "" {
		t.Fatal("register did not return a token")
	}
	cookie := res.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") {
		t.Fatalf("session cookie flags: %s", cookie)
	}
	if strings.Contains(cookie, "Secure") {
		t.Fatalf("plain HTTP test must not force Secure: %s", cookie)
	}
	if status(http.MethodPost, "/api/v1/auth/register", "", "", map[string]string{
		"email": "second@example.com", "name": "Second", "password": "password12345",
	}) != http.StatusForbidden {
		t.Fatal("registration stays open after the first account")
	}
	if status(http.MethodGet, "/api/v1/projects", "", "", nil) != http.StatusUnauthorized {
		t.Fatal("projects without a session")
	}

	health, _ := http.Get(srv.URL + "/health")
	if health.Header.Get("Content-Security-Policy") == "" || health.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers missing: %v", health.Header)
	}
	health.Body.Close()
	corsReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/version", nil)
	corsReq.Header.Set("Origin", "https://evil.example")
	corsRes, err := http.DefaultClient.Do(corsReq)
	if err != nil {
		t.Fatal(err)
	}
	corsRes.Body.Close()
	if got := corsRes.Header.Get("Access-Control-Allow-Origin"); got == "https://evil.example" || got == "*" {
		t.Fatalf("reflected origin %q", got)
	}

	res, parsed = do(http.MethodPost, "/api/v1/projects", ownerToken, "", map[string]string{
		"name": "'; DROP TABLE users; --", "description": "a",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create project %d %v", res.StatusCode, parsed)
	}
	proj := parsed["project"].(map[string]any)
	env := parsed["environment"].(map[string]any)
	projectID := proj["id"].(string)
	envID := env["id"].(string)
	var users int
	if err := st.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM users`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("users after hostile project name: %d %v", users, err)
	}

	res, parsed = do(http.MethodPost, "/api/v1/projects", ownerToken, "", map[string]string{"name": "other"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("second project %d %v", res.StatusCode, parsed)
	}
	otherEnv := parsed["environment"].(map[string]any)["id"].(string)

	res, parsed = do(http.MethodPost, "/api/v1/applications", ownerToken, "", map[string]string{
		"environment_id": envID, "name": "web", "build_pack": "dockerfile", "dockerfile": "FROM nginx:alpine\n",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create app %d %v", res.StatusCode, parsed)
	}
	appID := parsed["id"].(string)
	if appID == "" {
		if app, ok := parsed["application"].(map[string]any); ok {
			appID, _ = app["id"].(string)
		}
	}
	if appID == "" {
		t.Fatalf("no application id in %v", parsed)
	}
	res, parsed = do(http.MethodPost, "/api/v1/applications", ownerToken, "", map[string]string{
		"environment_id": otherEnv, "name": "hidden", "build_pack": "dockerfile",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("hidden app %d %v", res.StatusCode, parsed)
	}
	hiddenID, _ := parsed["id"].(string)

	if status(http.MethodPost, "/api/v1/webhooks/git/"+appID, "", "", map[string]any{"ref": "refs/heads/main"}) != http.StatusUnauthorized {
		t.Fatal("unsigned webhook accepted")
	}

	ctx := t.Context()
	member, memberTeam, err := st.CreateUserWithPersonalTeam(ctx, "viewer@example.com", "Viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ownerID, err := uuid.Parse(parsedUserID(t, st, "owner@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	_ = ownerID
	ownerTeam := mustTeam(t, st, "owner@example.com")
	if _, err := st.Pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, access_scope) VALUES ($1,$2,'member','projects')`, ownerTeam, member.ID); err != nil {
		t.Fatal(err)
	}
	pid, _ := uuid.Parse(projectID)
	if err := st.SetProjectAccess(ctx, ownerTeam, member.ID, "projects", []store.ProjectGrant{{ProjectID: pid, Permission: "view"}}); err != nil {
		t.Fatal(err)
	}
	viewToken, err := crypto.RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, member.ID, ownerTeam, viewToken, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if status(http.MethodGet, "/api/v1/applications/"+appID, viewToken, "", nil) != http.StatusOK {
		t.Fatal("viewer cannot read an allowed application")
	}
	if status(http.MethodGet, "/api/v1/applications/"+hiddenID, viewToken, "", nil) != http.StatusNotFound {
		t.Fatal("viewer can read an application outside their projects")
	}
	if got := status(http.MethodPatch, "/api/v1/applications/"+appID, viewToken, "", map[string]string{"name": "renamed"}); got != http.StatusNotFound && got != http.StatusForbidden {
		t.Fatalf("viewer patch status %d", got)
	}
	if got := status(http.MethodDelete, "/api/v1/applications/"+appID, viewToken, "", map[string]string{"confirmation_name": "web", "password": "x"}); got != http.StatusNotFound && got != http.StatusForbidden {
		t.Fatalf("viewer delete status %d", got)
	}
	if status(http.MethodGet, "/api/v1/env-vars?resource_type=application&resource_id="+hiddenID, viewToken, "", nil) != http.StatusNotFound {
		t.Fatal("viewer listed env vars for a hidden application")
	}
	if status(http.MethodGet, "/api/v1/projects", viewToken, memberTeam.ID.String(), nil) != http.StatusOK {
		t.Fatal("member could not list projects on their own team")
	}
	res, parsed = do(http.MethodGet, "/api/v1/projects", viewToken, memberTeam.ID.String(), nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("own team projects %d %v", res.StatusCode, parsed)
	}
	raw, _ := json.Marshal(parsed)
	if strings.Contains(string(raw), projectID) {
		t.Fatal("X-Team-ID to the member's own team leaked the owner's project")
	}
	if status(http.MethodGet, "/api/v1/projects", viewToken, uuid.NewString(), nil) != http.StatusForbidden {
		t.Fatal("unknown team was accepted")
	}

	big := bytes.Repeat([]byte("a"), 2<<20)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/projects", bytes.NewReader(big))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	bigRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	bigRes.Body.Close()
	if bigRes.StatusCode != http.StatusBadRequest && bigRes.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body status %d", bigRes.StatusCode)
	}
}

func parsedUserID(t *testing.T, st *store.Store, email string) string {
	t.Helper()
	u, _, err := st.GetUserByEmail(t.Context(), email)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID.String()
}

func mustTeam(t *testing.T, st *store.Store, email string) uuid.UUID {
	t.Helper()
	u, _, err := st.GetUserByEmail(t.Context(), email)
	if err != nil {
		t.Fatal(err)
	}
	teams, err := st.ListTeamsForUser(t.Context(), u.ID)
	if err != nil || len(teams) == 0 {
		t.Fatal(err)
	}
	return teams[0].ID
}

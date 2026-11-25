package storetests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestServersRegistriesAuditAndLimits(t *testing.T) {
	w := newWorld(t, "ops@example.com", "ops")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID
	app := w.app(t, "web", "dockerfile")

	srv, err := st.CreateServer(ctx, teamID, nil, "edge", "primary", "203.0.113.10", "root", 22, "traefik")
	if err != nil || srv.IP != "203.0.113.10" {
		t.Fatalf("%+v %v", srv, err)
	}
	if err := st.UpdateServerStatus(ctx, srv.ID, true, true, "29.0.0", "running"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateServerReachability(ctx, srv.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateServerProxyStatus(ctx, srv.ID, "running"); err != nil {
		t.Fatal(err)
	}
	dest, err := st.CreateDestination(ctx, teamID, srv.ID, "extra", "standalone", "dockfin-extra")
	if err != nil || dest.ServerID != srv.ID {
		t.Fatalf("%+v %v", dest, err)
	}
	got, err := st.GetDestination(ctx, teamID, dest.ID)
	if err != nil || got.Name != "extra" || got.Network != "dockfin-extra" {
		t.Fatalf("%+v %v", got, err)
	}
	if err := st.InsertServerMetric(ctx, teamID, srv.ID, store.ServerMetric{
		CPUPercent: 12, MemoryUsedBytes: 1, MemoryTotalBytes: 4, DiskUsedBytes: 2, DiskTotalBytes: 8, RecordedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	metrics, err := st.ListServerMetrics(ctx, teamID, srv.ID, 10)
	if err != nil || len(metrics) != 1 || metrics[0].CPUPercent != 12 {
		t.Fatalf("%+v %v", metrics, err)
	}
	disk := 40
	if err := st.SaveServerHealth(ctx, store.ServerHealth{
		ServerID: srv.ID, TeamID: teamID, Reachable: true, DiskUsedPercent: &disk, DiskMount: "/", TraefikImage: "traefik:v3",
	}); err != nil {
		t.Fatal(err)
	}
	health, err := st.GetServerHealth(ctx, teamID, srv.ID)
	if err != nil || !health.Reachable || health.DiskMount != "/" {
		t.Fatalf("%+v %v", health, err)
	}
	if err := st.SetServerOpsSettings(ctx, teamID, srv.ID, store.ServerOpsSettings{
		DockerCleanupFrequency: "0 4 * * *", DockerCleanupThreshold: 80, DiskUsageNotificationThreshold: 90,
		TrafficIPMode: "anonymized", TrafficRetentionDays: 7,
	}); err != nil {
		t.Fatal(err)
	}
	n, err := st.ServerResourceCount(ctx, srv.ID)
	if err != nil || n < 0 {
		t.Fatal(n, err)
	}
	exp, err := st.ExportServer(ctx, teamID, srv.ID)
	if err != nil || exp.Name != "edge" || exp.IP != "203.0.113.10" {
		t.Fatalf("%+v %v", exp, err)
	}
	imported, err := st.ImportServer(ctx, teamID, nil, *exp)
	if err != nil || imported.IP != exp.IP {
		t.Fatalf("%+v %v", imported, err)
	}

	reg, err := st.CreateDockerRegistry(ctx, teamID, "ghcr", "ghcr.io", "user", "enc")
	if err != nil || reg.URL != "ghcr.io" {
		t.Fatalf("%+v %v", reg, err)
	}
	url := "https://index.docker.io"
	updated, err := st.UpdateDockerRegistry(ctx, teamID, reg.ID, reg.Name, url, reg.Username, nil)
	if err != nil || updated.URL != url {
		t.Fatalf("%+v %v", updated, err)
	}

	uid := w.user.ID
	if err := st.InsertAuditLog(ctx, store.AuditLog{
		TeamID: teamID, UserID: &uid, Method: "POST", Path: "/api/v1/projects", Action: "create",
		Category: "resource", Summary: "created project", StatusCode: 201,
	}); err != nil {
		t.Fatal(err)
	}
	logs, err := st.ListAuditLogs(ctx, teamID, 10)
	if err != nil || len(logs) == 0 || logs[0].Summary != "created project" {
		t.Fatalf("%+v %v", logs, err)
	}
	page, err := st.QueryAuditLogs(ctx, store.AuditQuery{TeamID: teamID, Limit: 10, Category: "resource"})
	if err != nil || len(page.Logs) == 0 {
		t.Fatalf("%+v %v", page, err)
	}

	ok, _, err := st.AllowRateLimit(ctx, "login:"+w.user.Email, 2, time.Minute)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := st.PruneRateLimitHits(ctx); err != nil {
		t.Fatal(err)
	}

	hookID := int64(9)
	hook, err := st.CreateGitLabSystemHook(ctx, teamID, "https://gitlab.example.com", "hash", "enc", &hookID)
	if err != nil || hook.GitLabURL != "https://gitlab.example.com" {
		t.Fatalf("%+v %v", hook, err)
	}
	found, _, err := st.GitLabSystemHookByToken(ctx, "hash")
	if err != nil || found.ID != hook.ID {
		t.Fatalf("%+v %v", found, err)
	}

	tok, err := st.CreateIntegrationToken(ctx, teamID, "infisical", "prod", "enc-token", json.RawMessage(`{"project":"p"}`))
	if err != nil || tok.Provider != "infisical" {
		t.Fatalf("%+v %v", tok, err)
	}
	if _, err := st.GetIntegrationToken(ctx, teamID, tok.ID); err != nil {
		t.Fatal(err)
	}

	if err := st.UpdateApplication(ctx, app); err != nil {
		t.Fatal(err)
	}
	app.GitRepository = "https://github.com/acme/web.git"
	app.GitBranch = "main"
	if err := st.UpdateApplication(ctx, app); err != nil {
		t.Fatal(err)
	}
	matches, err := st.ListApplicationsForGitWebhook(ctx, uuid.Nil, "acme/web", "main")
	if err != nil {
		t.Fatal(err)
	}
	_ = matches
	if _, err := st.ListEnabledScheduledTasks(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListEnabledScheduledBackups(ctx); err != nil {
		t.Fatal(err)
	}
	limit, err := st.GetDeploymentQueueLimit(ctx, srv.ID)
	if err != nil || limit < 0 {
		t.Fatal(limit, err)
	}
	active, err := st.CountActiveDeployments(ctx, srv.ID)
	if err != nil || active != 0 {
		t.Fatal(active, err)
	}
}

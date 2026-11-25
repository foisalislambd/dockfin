package storetests

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/traffic"
	"github.com/google/uuid"
)

func TestTrafficReportsAndHosts(t *testing.T) {
	w := newWorld(t, "traffic@example.com", "traffic")
	ctx := t.Context()
	st := w.st
	srv, err := st.CreateServer(ctx, w.team.ID, nil, "edge", "", "203.0.113.10", "root", 22, "traefik")
	if err != nil || srv.IP != "203.0.113.10" {
		t.Fatalf("server %+v %v", srv, err)
	}
	dests, err := st.ListDestinations(ctx, w.team.ID, &srv.ID)
	if err != nil || len(dests) != 1 || dests[0].Network != "dockfin" {
		t.Fatalf("destinations %+v %v", dests, err)
	}
	app := w.app(t, "web", "dockerfile")
	app.FQDN = "https://App.Example, https://pr.example"
	app.DestinationID = &dests[0].ID
	if err := st.UpdateApplication(ctx, app); err != nil {
		t.Fatal(err)
	}
	preview, err := st.CreatePreview(ctx, w.team.ID, app.ID, 7, "fix", "pr-7", "https://pr.example")
	if err != nil || preview.PullRequestID != 7 || preview.FQDN != "https://pr.example" {
		t.Fatalf("preview %+v %v", preview, err)
	}

	now := time.Now().UTC()
	rows := []traffic.Counted{
		{Hit: traffic.Hit{Host: "old.example", Path: "/", Method: "GET", Class: "2xx", Client: "203.0.113.9", At: now.Add(-10 * 24 * time.Hour)}, N: 4},
		{Hit: traffic.Hit{Host: "app.example", Path: "/health", Method: "GET", Class: "2xx", Client: "203.0.113.9", At: now}, N: 3},
		{Hit: traffic.Hit{Host: "app.example", Path: "/login", Method: "POST", Class: "4xx", Client: "203.0.113.9", At: now}, N: 1},
		{Hit: traffic.Hit{Host: "", Path: "/", Method: "GET", Class: "2xx", At: now}, N: 9},
		{Hit: traffic.Hit{Host: "skip.example", Path: "/", Method: "GET", Class: "2xx", At: now}, N: 0},
	}
	if err := st.AddTraffic(ctx, srv.ID, now, rows, "anonymized", 7); err != nil {
		t.Fatal(err)
	}
	since := now.Add(-2 * time.Hour)
	report, err := st.TrafficReport(ctx, srv.ID, since, nil, "hour")
	if err != nil {
		t.Fatal(err)
	}
	if report.Requests != 4 {
		t.Fatalf("requests %d", report.Requests)
	}
	if len(report.Hosts) != 1 || report.Hosts[0].Name != "app.example" || report.Hosts[0].Requests != 4 {
		t.Fatalf("hosts %+v", report.Hosts)
	}
	if len(report.Paths) != 2 || report.Paths[0].Path != "/health" || report.Paths[0].Requests != 3 {
		t.Fatalf("paths %+v", report.Paths)
	}
	if len(report.Statuses) != 2 || report.Statuses[0].Name != "2xx" || report.Statuses[0].Requests != 3 {
		t.Fatalf("statuses %+v", report.Statuses)
	}
	if len(report.Clients) != 1 || report.Clients[0].Name != "203.0.113.0/24" || report.Clients[0].Requests != 4 {
		t.Fatalf("clients %+v", report.Clients)
	}
	if len(report.Series) == 0 || report.Series[0].Class == "" || report.Series[0].Requests == 0 {
		t.Fatalf("series %+v", report.Series)
	}
	filtered, err := st.TrafficReport(ctx, srv.ID, since, []string{"app.example"}, "day")
	if err != nil || filtered.Requests != 4 {
		t.Fatalf("filtered %+v %v", filtered, err)
	}
	none, err := st.TrafficReport(ctx, srv.ID, since, []string{}, "hour")
	if err != nil || none.Requests != 0 || len(none.Hosts) != 0 {
		t.Fatalf("empty host list %+v %v", none, err)
	}
	if err := st.ClearTrafficClients(ctx, srv.ID); err != nil {
		t.Fatal(err)
	}
	cleared, err := st.TrafficReport(ctx, srv.ID, since, nil, "hour")
	if err != nil || cleared.Requests != 4 || len(cleared.Clients) != 0 {
		t.Fatalf("cleared %+v %v", cleared, err)
	}
	if err := st.AddTraffic(ctx, srv.ID, now, []traffic.Counted{{
		Hit: traffic.Hit{Host: "app.example", Path: "/health", Method: "GET", Class: "2xx", Client: "198.51.100.8", At: now}, N: 2,
	}}, "off", 7); err != nil {
		t.Fatal(err)
	}
	off, err := st.TrafficReport(ctx, srv.ID, since, nil, "hour")
	if err != nil || off.Requests != 6 || len(off.Clients) != 0 {
		t.Fatalf("ip mode off %+v %v", off, err)
	}

	hosts, serverID, err := st.ApplicationHosts(ctx, w.team.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if serverID != srv.ID || len(hosts) != 2 || hosts[0] != "app.example" || hosts[1] != "pr.example" {
		t.Fatalf("app hosts %v server %s", hosts, serverID)
	}
	svc, err := st.CreateService(ctx, &store.Service{
		TeamID: w.team.ID, EnvironmentID: w.env.ID, ServerID: &srv.ID, Name: "n8n", ServiceType: "n8n",
		DockerComposeRaw: "services:\n  n8n:\n    image: n8nio/n8n\n", FQDN: "https://svc.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServiceDomain(ctx, w.team.ID, svc.ID, store.ServiceDomainInput{
		UnitName: "n8n", Host: "extra.example", Scheme: "https", Redirect: "both",
	}); err != nil {
		t.Fatal(err)
	}
	shosts, sid, err := st.ServiceHosts(ctx, w.team.ID, svc.ID)
	if err != nil || sid != srv.ID || len(shosts) != 2 || shosts[0] != "svc.example" || shosts[1] != "extra.example" {
		t.Fatalf("service hosts %v %s %v", shosts, sid, err)
	}
}

func TestMaintenanceControls(t *testing.T) {
	w := newWorld(t, "maint@example.com", "maint")
	ctx := t.Context()
	st := w.st
	srv, err := st.CreateServer(ctx, w.team.ID, nil, "edge", "", "203.0.113.20", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	dests, err := st.ListDestinations(ctx, w.team.ID, &srv.ID)
	if err != nil || len(dests) != 1 {
		t.Fatal(dests, err)
	}
	app := w.app(t, "web", "dockerfile")
	app.FQDN = "https://web.example"
	app.DestinationID = &dests[0].ID
	if err := st.UpdateApplication(ctx, app); err != nil {
		t.Fatal(err)
	}
	empty, err := st.GetResourceControl(ctx, w.team.ID, "application", app.ID)
	if err != nil || empty.MaintenanceEnabled || empty.ResourceID != app.ID {
		t.Fatalf("empty control %+v %v", empty, err)
	}
	if err := st.UpsertResourceControl(ctx, store.ResourceControl{
		TeamID: w.team.ID, ResourceType: "application", ResourceID: app.ID,
		MaintenanceEnabled: true, MaintenanceMessage: "brb", RedirectPermanent: true,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetResourceControl(ctx, w.team.ID, "application", app.ID)
	if err != nil || !got.MaintenanceEnabled || got.MaintenanceMessage != "brb" || !got.RedirectPermanent {
		t.Fatalf("control %+v %v", got, err)
	}
	svc, err := st.CreateService(ctx, &store.Service{
		TeamID: w.team.ID, EnvironmentID: w.env.ID, ServerID: &srv.ID, Name: "queue", ServiceType: "n8n",
		DockerComposeRaw: "services:\n  n8n:\n    image: n8nio/n8n\n", FQDN: "queue.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertResourceControl(ctx, store.ResourceControl{
		TeamID: w.team.ID, ResourceType: "service", ResourceID: svc.ID,
		MaintenanceEnabled: true, MaintenanceMessage: "paused",
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.GetService(ctx, w.team.ID, svc.ID)
	if err != nil || !loaded.MaintenanceEnabled || loaded.MaintenanceMessage != "paused" {
		t.Fatalf("service %+v %v", loaded, err)
	}
	controls, err := st.ListMaintenanceControls(ctx, w.team.ID)
	if err != nil || len(controls) != 2 {
		t.Fatalf("controls %+v %v", controls, err)
	}
	targets, err := st.ListServerMaintenance(ctx, w.team.ID, srv.ID)
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets %+v %v", targets, err)
	}
	byType := map[string]store.MaintenanceTarget{}
	for _, target := range targets {
		byType[target.ResourceType] = target
	}
	appTarget := byType["application"]
	if appTarget.ResourceID != app.ID || appTarget.FQDN != "https://web.example" || appTarget.Message != "brb" || !appTarget.ForceHTTPS {
		t.Fatalf("app target %+v", appTarget)
	}
	svcTarget := byType["service"]
	if svcTarget.ResourceID != svc.ID || svcTarget.FQDN != "queue.example" || svcTarget.Message != "paused" {
		t.Fatalf("service target %+v", svcTarget)
	}
}

func TestAuditFiltersAndPrune(t *testing.T) {
	w := newWorld(t, "audit@example.com", "audit")
	ctx := t.Context()
	st := w.st
	uid := w.user.ID
	if err := st.InsertAuditLog(ctx, store.AuditLog{
		TeamID: w.team.ID, UserID: &uid, Method: "POST", Path: "/api/v1/projects", Action: "create",
		Category: "resource", Summary: "created project", ResourceType: "project", ResourceID: w.project.ID.String(),
		StatusCode: 201, IP: "203.0.113.4", ActorEmail: "ops@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertAuditLog(ctx, store.AuditLog{
		TeamID: w.team.ID, Method: "DELETE", Path: "/api/v1/projects/" + w.project.ID.String(), Action: "delete",
		Category: "resource", Summary: "deleted draft", ResourceType: "project", StatusCode: 204,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertAuditLog(ctx, store.AuditLog{
		Method: "POST", Path: "/login", Action: "login", Category: "auth", Summary: "unknown login", StatusCode: 401,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertAuditLog(ctx, store.AuditLog{
		TeamID: w.team.ID, Method: "POST", Path: "/old", Action: "create", Category: "resource", Summary: "old event", StatusCode: 201,
	}); err != nil {
		t.Fatal(err)
	}

	page, err := st.QueryAuditLogs(ctx, store.AuditQuery{
		TeamID: w.team.ID, Action: "create", Category: "resource", ResourceType: "project", Query: "created", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Limit != 10 || len(page.Logs) != 1 {
		t.Fatalf("page %+v", page)
	}
	row := page.Logs[0]
	if row.Summary != "created project" || row.ActorEmail != "ops@example.com" || row.UserEmail != w.user.Email || row.StatusCode != 201 || row.IP != "203.0.113.4" {
		t.Fatalf("row %+v", row)
	}
	miss, err := st.QueryAuditLogs(ctx, store.AuditQuery{TeamID: w.team.ID, Query: "created_project", Limit: 10})
	if err != nil || miss.Total != 0 {
		t.Fatalf("underscore query %+v %v", miss, err)
	}
	scoped, err := st.QueryAuditLogs(ctx, store.AuditQuery{TeamID: w.team.ID, Limit: 20})
	if err != nil || scoped.Total != 3 {
		t.Fatalf("scoped %+v %v", scoped, err)
	}
	listed, err := st.ListAuditLogs(ctx, w.team.ID, 20)
	if err != nil || len(listed) != 4 {
		t.Fatalf("listed %d %v", len(listed), err)
	}
	if listed[0].Summary == "" {
		t.Fatalf("listed %+v", listed)
	}
	foundUnknown := false
	for _, log := range listed {
		if log.Summary == "unknown login" && log.TeamID == uuid.Nil && log.Category == "auth" {
			foundUnknown = true
		}
	}
	if !foundUnknown {
		t.Fatalf("unscoped auth missing %+v", listed)
	}

	tag, err := st.Pool.Exec(ctx, `UPDATE audit_logs SET created_at = NOW() - INTERVAL '30 days' WHERE summary = 'old event'`)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal(tag, err)
	}
	kept, err := st.PruneAuditLogs(ctx, 0)
	if err != nil || kept != 0 {
		t.Fatalf("keep forever %d %v", kept, err)
	}
	n, err := st.PruneAuditLogs(ctx, 7)
	if err != nil || n != 1 {
		t.Fatalf("pruned %d %v", n, err)
	}
	after, err := st.QueryAuditLogs(ctx, store.AuditQuery{TeamID: w.team.ID, Query: "old event", Limit: 10})
	if err != nil || after.Total != 0 {
		t.Fatalf("after prune %+v %v", after, err)
	}
	remain, err := st.QueryAuditLogs(ctx, store.AuditQuery{TeamID: w.team.ID, Limit: 10})
	if err != nil || remain.Total != 2 {
		t.Fatalf("remain %+v %v", remain, err)
	}
}

func TestSharedEnvResolves(t *testing.T) {
	w := newWorld(t, "shared@example.com", "shared")
	ctx := t.Context()
	st := w.st
	app := w.app(t, "web", "dockerfile")
	teamVar, err := st.UpsertSharedEnv(ctx, w.team.ID, "team", nil, "DB_USER", "app", false)
	if err != nil || teamVar.Key != "DB_USER" || teamVar.Value != "app" || teamVar.ScopeID != nil {
		t.Fatalf("team var %+v %v", teamVar, err)
	}
	projectID := w.project.ID
	projectVar, err := st.UpsertSharedEnv(ctx, w.team.ID, "project", &projectID, "REGION", "eu", true)
	if err != nil || projectVar.Value != "eu" || !projectVar.IsLiteral || projectVar.ScopeID == nil || *projectVar.ScopeID != projectID {
		t.Fatalf("project var %+v %v", projectVar, err)
	}
	updated, err := st.UpsertSharedEnv(ctx, w.team.ID, "team", nil, "DB_USER", "app2", false)
	if err != nil || updated.ID != teamVar.ID || updated.Value != "app2" {
		t.Fatalf("updated %+v %v", updated, err)
	}
	hidden, err := st.ListSharedEnv(ctx, w.team.ID, "team", nil, false)
	if err != nil || len(hidden) != 1 || hidden[0].Key != "DB_USER" || hidden[0].Value != "" {
		t.Fatalf("hidden %+v %v", hidden, err)
	}
	shown, err := st.ListSharedEnv(ctx, w.team.ID, "project", &projectID, true)
	if err != nil || len(shown) != 1 || shown[0].Value != "eu" || !shown[0].IsLiteral {
		t.Fatalf("shown %+v %v", shown, err)
	}
	if _, err := st.UpsertEnvVar(ctx, w.team.ID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "DATABASE_URL", Value: "postgres://{{team.DB_USER}}@{{project.REGION}}/db", Runtime: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, w.team.ID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "LITERAL_URL", Value: "{{team.DB_USER}}", Runtime: true, Literal: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, w.team.ID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "DATABASE_URL", Value: "preview-db", Runtime: true, IsPreview: true,
	}); err != nil {
		t.Fatal(err)
	}
	envID := w.env.ID
	resolved, err := st.ResolvedEnvMap(ctx, w.team.ID, "application", app.ID, &projectID, &envID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["DATABASE_URL"] != "postgres://app2@eu/db" || resolved["LITERAL_URL"] != "{{team.DB_USER}}" {
		t.Fatalf("resolved %#v", resolved)
	}
	preview, err := st.ResolvedEnvMapPreview(ctx, w.team.ID, "application", app.ID, &projectID, &envID, nil)
	if err != nil || preview["DATABASE_URL"] != "preview-db" {
		t.Fatalf("preview %#v %v", preview, err)
	}
}

func TestVolumesCloneAndContent(t *testing.T) {
	w := newWorld(t, "volumes@example.com", "volumes")
	ctx := t.Context()
	st := w.st
	app := w.app(t, "web", "dockerfile")
	body := "secret-file"
	file, err := st.UpsertVolume(ctx, w.team.ID, "application", app.ID, "config", "etc/app.conf", "/data/"+app.ID.String()+"/conf", true, &body)
	if err != nil || file.MountPath != "/etc/app.conf" || !file.IsFile || !file.HasContent || file.Name != "config" {
		t.Fatalf("file %+v %v", file, err)
	}
	plain, err := st.VolumeContent(ctx, w.team.ID, file.ID)
	if err != nil || plain != body {
		t.Fatalf("content %q %v", plain, err)
	}
	again, err := st.UpsertVolume(ctx, w.team.ID, "application", app.ID, "config", "/etc/new.conf", "", true, nil)
	if err != nil || again.ID != file.ID || again.MountPath != "/etc/new.conf" || again.HostPath != "" || !again.HasContent {
		t.Fatalf("update %+v %v", again, err)
	}
	if plain, err = st.VolumeContent(ctx, w.team.ID, file.ID); err != nil || plain != body {
		t.Fatalf("kept content %q %v", plain, err)
	}
	dirBody := "ignored"
	dir, err := st.UpsertVolume(ctx, w.team.ID, "application", app.ID, "data", "/data", "/var/lib/data", false, &dirBody)
	if err != nil || dir.IsFile || dir.HasContent || dir.HostPath != "/var/lib/data" {
		t.Fatalf("dir %+v %v", dir, err)
	}
	if got, err := st.VolumeContent(ctx, w.team.ID, dir.ID); err != nil || got != "" {
		t.Fatalf("dir content %q %v", got, err)
	}
	listed, err := st.ListVolumes(ctx, w.team.ID, "application", app.ID)
	if err != nil || len(listed) != 2 || listed[0].Name != "config" || listed[1].Name != "data" {
		t.Fatalf("list %+v %v", listed, err)
	}
	other := w.app(t, "copy", "dockerfile")
	if err := st.CloneVolumes(ctx, w.team.ID, "application", app.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	cloned, err := st.ListVolumes(ctx, w.team.ID, "application", other.ID)
	if err != nil || len(cloned) != 2 {
		t.Fatalf("cloned %+v %v", cloned, err)
	}
	for _, vol := range cloned {
		if vol.HostPath != "" {
			t.Fatalf("host path kept %+v", vol)
		}
		if vol.Name == "config" {
			got, err := st.VolumeContent(ctx, w.team.ID, vol.ID)
			if err != nil || got != body || !vol.HasContent {
				t.Fatalf("cloned file %q %+v %v", got, vol, err)
			}
		}
	}
	if err := st.DeleteVolume(ctx, w.team.ID, dir.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetVolume(ctx, w.team.ID, dir.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted volume err %v", err)
	}
	if err := st.DeleteVolume(ctx, w.team.ID, dir.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete %v", err)
	}
	if _, err := st.UpsertVolume(ctx, w.team.ID, "application", app.ID, " ", "/tmp", "", false, nil); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatal(err)
	}
}

func TestPreviewFlagAndFreeze(t *testing.T) {
	w := newWorld(t, "freeze@example.com", "freeze")
	ctx := t.Context()
	st := w.st
	app := w.app(t, "web", "dockerfile")
	if app.IsPreviewEnabled {
		t.Fatal("preview enabled by default")
	}
	if err := st.SetApplicationPreviewEnabled(ctx, w.team.ID, app.ID, true); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.GetApplication(ctx, w.team.ID, app.ID)
	if err != nil || !loaded.IsPreviewEnabled {
		t.Fatalf("preview %+v %v", loaded, err)
	}
	clone, err := st.CloneApplication(ctx, w.team.ID, app.ID, "web-preview", nil)
	if err != nil || clone.Name != "web-preview" || !clone.IsPreviewEnabled || clone.FQDN != "" || clone.ID == app.ID {
		t.Fatalf("clone %+v %v", clone, err)
	}
	if err := st.SetApplicationPreviewEnabled(ctx, w.team.ID, app.ID, false); err != nil {
		t.Fatal(err)
	}
	loaded, err = st.GetApplication(ctx, w.team.ID, app.ID)
	if err != nil || loaded.IsPreviewEnabled {
		t.Fatalf("preview still on %+v %v", loaded, err)
	}
	if clone, err = st.GetApplication(ctx, w.team.ID, clone.ID); err != nil || !clone.IsPreviewEnabled {
		t.Fatalf("clone flag changed %+v %v", clone, err)
	}

	uid := w.user.ID
	frozen, err := st.SetEnvironmentDeployFreeze(ctx, w.team.ID, w.project.ID, w.env.ID, &uid, true, "holiday deploy")
	if err != nil {
		t.Fatal(err)
	}
	if !frozen.DeployFrozen || frozen.DeployFreezeReason != "holiday deploy" || frozen.DeployFrozenBy == nil || *frozen.DeployFrozenBy != uid || frozen.DeployFrozenAt == nil || frozen.IsEmpty {
		t.Fatalf("frozen %+v", frozen)
	}
	open, err := st.SetEnvironmentDeployFreeze(ctx, w.team.ID, w.project.ID, w.env.ID, &uid, false, "ignored")
	if err != nil || open.DeployFrozen || open.DeployFreezeReason != "" || open.DeployFrozenBy != nil || open.DeployFrozenAt != nil {
		t.Fatalf("unfrozen %+v %v", open, err)
	}
	if _, err := st.SetEnvironmentDeployFreeze(ctx, w.team.ID, w.project.ID, uuid.New(), &uid, true, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestTransferAndMoveWithoutSSH(t *testing.T) {
	w := newWorld(t, "move@example.com", "move")
	ctx := t.Context()
	st := w.st
	app := w.app(t, "web", "dockerfile")
	db := w.database(t, "pg")
	srv, err := st.CreateServer(ctx, w.team.ID, nil, "edge", "primary", "203.0.113.30", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	dests, err := st.ListDestinations(ctx, w.team.ID, &srv.ID)
	if err != nil || len(dests) != 1 {
		t.Fatal(dests, err)
	}
	app.DestinationID = &dests[0].ID
	app.FQDN = "web.example"
	if err := st.UpdateApplication(ctx, app); err != nil {
		t.Fatal(err)
	}
	staging, err := st.CreateEnvironment(ctx, w.team.ID, w.project.ID, "staging", "pre")
	if err != nil || staging.Name != "staging" || staging.ProjectID != w.project.ID {
		t.Fatalf("staging %+v %v", staging, err)
	}
	if err := st.MoveResource(ctx, w.team.ID, "database", db.ID, staging.ID); err != nil {
		t.Fatal(err)
	}
	movedDB, err := st.GetDatabase(ctx, w.team.ID, db.ID)
	if err != nil || movedDB.EnvironmentID != staging.ID {
		t.Fatalf("moved db %+v %v", movedDB, err)
	}
	if err := st.MoveResource(ctx, w.team.ID, "database", db.ID, staging.ID); err != nil {
		t.Fatal(err)
	}
	otherProject, otherEnv, err := st.CreateProject(ctx, w.team.ID, "other", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MoveResource(ctx, w.team.ID, "database", db.ID, otherEnv.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatal(err)
	}
	_ = otherProject

	uid := w.user.ID
	frozen, err := st.SetEnvironmentDeployFreeze(ctx, w.team.ID, w.project.ID, w.env.ID, &uid, true, "change window")
	if err != nil || !frozen.DeployFrozen || frozen.DeployFreezeReason != "change window" {
		t.Fatalf("freeze %+v %v", frozen, err)
	}
	target, err := st.CreateTeam(ctx, w.user.ID, "destination", "other team")
	if err != nil || target.Name != "destination" {
		t.Fatalf("team %+v %v", target, err)
	}
	if _, err := st.TransferServer(ctx, w.team.ID, w.team.ID, srv.ID); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatal(err)
	}
	result, err := st.TransferServer(ctx, w.team.ID, target.ID, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerID != srv.ID || result.Applications != 1 || result.Databases != 0 || result.Services != 0 || result.Projects != 0 || result.Split != 1 {
		t.Fatalf("result %+v", result)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "another server") {
		t.Fatalf("warnings %+v", result.Warnings)
	}
	if _, err := st.GetServer(ctx, w.team.ID, srv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	gotSrv, err := st.GetServer(ctx, target.ID, srv.ID)
	if err != nil || gotSrv.Name != "edge" || gotSrv.IP != "203.0.113.30" || gotSrv.TeamID != target.ID {
		t.Fatalf("target server %+v %v", gotSrv, err)
	}
	if _, err := st.GetApplication(ctx, w.team.ID, app.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	movedApp, err := st.GetApplication(ctx, target.ID, app.ID)
	if err != nil || movedApp.Name != "web" || movedApp.EnvironmentID == w.env.ID || movedApp.FQDN != "web.example" {
		t.Fatalf("moved app %+v %v", movedApp, err)
	}
	targetEnv, err := st.GetEnvironment(ctx, target.ID, movedApp.EnvironmentID)
	if err != nil || targetEnv.DeployFrozen || targetEnv.Name == "" || targetEnv.TeamID != target.ID {
		t.Fatalf("target env %+v %v", targetEnv, err)
	}
	sourceEnv, err := st.GetEnvironment(ctx, w.team.ID, w.env.ID)
	if err != nil || !sourceEnv.DeployFrozen || sourceEnv.DeployFreezeReason != "change window" || !sourceEnv.IsEmpty {
		t.Fatalf("source env %+v %v", sourceEnv, err)
	}
	if stayed, err := st.GetDatabase(ctx, w.team.ID, db.ID); err != nil || stayed.EnvironmentID != staging.ID {
		t.Fatalf("database left behind %+v %v", stayed, err)
	}
}

func TestWebhookAndIntegrationTokens(t *testing.T) {
	w := newWorld(t, "tokens@example.com", "tokens")
	ctx := t.Context()
	st := w.st
	app := w.app(t, "web", "dockerfile")
	if secret, err := st.GetWebhookSecret(ctx, app.ID); err != nil || secret != "" {
		t.Fatalf("empty secret %q %v", secret, err)
	}
	if err := st.SetWebhookSecret(ctx, w.team.ID, app.ID, "hook-secret"); err != nil {
		t.Fatal(err)
	}
	secret, err := st.GetWebhookSecret(ctx, app.ID)
	if err != nil || secret != "hook-secret" {
		t.Fatalf("secret %q %v", secret, err)
	}
	if err := st.SetWebhookSecret(ctx, uuid.New(), app.ID, "other"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := st.GetWebhookSecret(ctx, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}

	type cred struct {
		provider string
		name     string
		plain    string
		meta     string
	}
	creds := []cred{
		{"doppler", "a-doppler", "dp.token", `{"project":"prod"}`},
		{"infisical", "b-infisical", "inf.token", `{"workspace":"w"}`},
		{"vault", "c-vault", "vault.token", `{"address":"https://vault.example"}`},
	}
	ids := map[string]uuid.UUID{}
	for _, c := range creds {
		tok, err := st.CreateIntegrationToken(ctx, w.team.ID, c.provider, c.name, encrypt(t, st, c.plain), []byte(c.meta))
		if err != nil || tok.Provider != c.provider || tok.Name != c.name {
			t.Fatalf("token %+v %v", tok, err)
		}
		ids[c.provider] = tok.ID
		provider, plain, meta, err := st.IntegrationTokenSecret(ctx, w.team.ID, tok.ID)
		if err != nil || provider != c.provider || plain != c.plain || !strings.Contains(string(meta), c.meta[2:8]) {
			t.Fatalf("secret %s %s %s %v", provider, plain, meta, err)
		}
	}
	listed, err := st.ListIntegrationTokens(ctx, w.team.ID)
	if err != nil || len(listed) != 3 || listed[0].Name != "a-doppler" || listed[2].Provider != "vault" {
		t.Fatalf("listed %+v %v", listed, err)
	}
	if _, err := st.CreateIntegrationToken(ctx, w.team.ID, "doppler", "a-doppler", encrypt(t, st, "again"), []byte(`{}`)); !errors.Is(err, store.ErrConflict) {
		t.Fatal(err)
	}
	link, err := st.UpsertSecretManagerLink(ctx, w.team.ID, "application", app.ID, ids["vault"], []byte(`{"path":"secret/data/app"}`))
	if err != nil || link.Provider != "vault" || link.TokenName != "c-vault" || link.ResourceID != app.ID {
		t.Fatalf("link %+v %v", link, err)
	}
	mat, err := st.SecretManagerMaterial(ctx, w.team.ID, "application", app.ID)
	if err != nil || mat.Provider != "vault" || mat.Token != "vault.token" || !strings.Contains(string(mat.Settings), "secret/data/app") {
		t.Fatalf("material %+v %v", mat, err)
	}
	renamed, err := st.UpdateIntegrationToken(ctx, w.team.ID, ids["vault"], "c-vault-2", nil, []byte(`{"address":"https://vault.example"}`))
	if err != nil || renamed.Name != "c-vault-2" {
		t.Fatalf("rename %+v %v", renamed, err)
	}
	if _, plain, _, err := st.IntegrationTokenSecret(ctx, w.team.ID, ids["vault"]); err != nil || plain != "vault.token" {
		t.Fatalf("secret rotated unexpectedly %q %v", plain, err)
	}
	next := encrypt(t, st, "vault.token.2")
	if _, err := st.UpdateIntegrationToken(ctx, w.team.ID, ids["vault"], "c-vault-2", &next, []byte(`{"address":"https://vault.example"}`)); err != nil {
		t.Fatal(err)
	}
	if _, plain, _, err := st.IntegrationTokenSecret(ctx, w.team.ID, ids["vault"]); err != nil || plain != "vault.token.2" {
		t.Fatalf("rotated %q %v", plain, err)
	}
	got, err := st.GetIntegrationToken(ctx, w.team.ID, ids["infisical"])
	if err != nil || got.Provider != "infisical" || got.Name != "b-infisical" {
		t.Fatalf("get %+v %v", got, err)
	}
	if err := st.DeleteSecretManagerLink(ctx, w.team.ID, "application", app.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SecretManagerMaterial(ctx, w.team.ID, "application", app.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if err := st.DeleteIntegrationToken(ctx, w.team.ID, ids["doppler"]); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetIntegrationToken(ctx, w.team.ID, ids["doppler"]); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

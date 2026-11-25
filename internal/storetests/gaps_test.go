package storetests

import (
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestStoreGapsAreReadable(t *testing.T) {
	w := newWorld(t, "gaps@example.com", "gaps")
	ctx := t.Context()
	app := w.app(t, "web", "dockerfile")
	server, err := w.st.CreateServer(ctx, w.team.ID, nil, "edge", "", "10.0.0.8", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}

	if err := w.st.SetAutoUpdateStatus(ctx, "ok", "idle"); err != nil {
		t.Fatal(err)
	}
	ran, err := w.st.AutoUpdateRanThisMinute(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_ = ran
	on, err := w.st.RegistrationEnabled(ctx)
	if err != nil || !on {
		t.Fatalf("registration %v %v", on, err)
	}
	if err := w.st.SetRegistrationEnabled(ctx, false); err != nil {
		t.Fatal(err)
	}
	if on, err = w.st.RegistrationEnabled(ctx); err != nil || on {
		t.Fatalf("registration stayed on %v %v", on, err)
	}
	if err := w.st.SetRegistrationEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}

	if err := w.st.UpsertResourceControl(ctx, store.ResourceControl{
		TeamID: w.team.ID, ResourceType: "application", ResourceID: app.ID,
		MaintenanceEnabled: true, MaintenanceMessage: "down", RedirectPermanent: true,
	}); err != nil {
		t.Fatal(err)
	}
	maint, err := w.st.ListMaintenanceControls(ctx, w.team.ID)
	if err != nil || len(maint) == 0 {
		t.Fatalf("controls %d %v", len(maint), err)
	}
	targets, err := w.st.ListServerMaintenance(ctx, w.team.ID, server.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = targets
	due, err := w.st.ListServersDueForHealth(ctx, time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	_ = due

	exp := time.Now().Add(time.Hour)
	tok, err := w.st.CreateApiToken(ctx, w.user.ID, w.team.ID, "ci", "plain-token-value", []string{"read"}, &exp)
	if err != nil {
		t.Fatal(err)
	}
	paused := true
	updated, err := w.st.UpdateApiToken(ctx, w.user.ID, w.team.ID, tok.ID, "ci2", []string{"read", "deploy"}, nil, true, &paused)
	if err != nil || updated.Name != "ci2" || updated.PausedAt == nil {
		t.Fatalf("%+v %v", updated, err)
	}
	rolled, err := w.st.RollApiToken(ctx, w.user.ID, w.team.ID, tok.ID, "rolled-token")
	if err != nil || rolled.ID != tok.ID {
		t.Fatal(err)
	}

	regs, err := w.st.ListDockerRegistries(ctx, w.team.ID)
	if err != nil || len(regs) != 0 {
		t.Fatal(err, len(regs))
	}
	s3, err := w.st.CreateS3Storage(ctx, w.team.ID, "minio", "http://127.0.0.1:9000", "b", "us-east-1", "ak", "sk", true)
	if err != nil {
		t.Fatal(err)
	}
	list, err := w.st.ListS3Storages(ctx, w.team.ID)
	if err != nil || len(list) != 1 || list[0].ID != s3.ID {
		t.Fatal(list, err)
	}
	got, err := w.st.GetS3Storage(ctx, w.team.ID, s3.ID)
	if err != nil || got.Bucket != "b" || !got.PathStyle {
		t.Fatal(got, err)
	}

	ids, err := w.st.ApplicationsForRepo(ctx, w.team.ID, "acme/web")
	if err != nil || len(ids) != 1 || ids[0] != app.ID {
		t.Fatalf("%v %v", ids, err)
	}
	if bad, err := w.st.ApplicationsForRepo(ctx, w.team.ID, "acme/%"); err != nil || bad != nil {
		t.Fatal(bad, err)
	}
	sources, err := w.st.ListGitSources(ctx, w.team.ID)
	if err != nil || len(sources) != 0 {
		t.Fatal(err)
	}
	hooks, err := w.st.ListGitLabSystemHooks(ctx, w.team.ID)
	if err != nil || len(hooks) != 0 {
		t.Fatal(err)
	}

	dep, err := w.st.CreateDeployment(ctx, w.team.ID, app.ID, &server.ID, "abc", "ok", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.st.SetDeploymentBuildServer(ctx, dep.ID, &server.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := w.st.GetDeployment(ctx, w.team.ID, dep.ID)
	if err != nil || loaded.ID != dep.ID {
		t.Fatalf("%+v %v", loaded, err)
	}
	var buildID uuid.UUID
	if err := w.st.Pool.QueryRow(ctx, `SELECT build_server_id FROM deployments WHERE id=$1`, dep.ID).Scan(&buildID); err != nil || buildID != server.ID {
		t.Fatal(buildID, err)
	}

	execs, err := w.st.ListScheduledTaskExecutionsForTeam(ctx, w.team.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	_ = execs
	svc := w.service(t, "redis")
	if err := w.st.SetServiceWebhookSecret(ctx, w.team.ID, svc.ID, "enc"); err != nil {
		t.Fatal(err)
	}
	has, err := w.st.ServiceHasWebhookSecret(ctx, w.team.ID, svc.ID)
	if err != nil || !has {
		t.Fatal(has, err)
	}
	secretTeam, secret, err := w.st.GetServiceWebhookSecret(ctx, svc.ID)
	if err != nil || secret != "enc" || secretTeam != w.team.ID {
		t.Fatal(secretTeam, secret, err)
	}
	byID, err := w.st.GetServiceByID(ctx, svc.ID)
	if err != nil || byID.Name != "redis" {
		t.Fatal(err)
	}

	extra, _, err := w.st.CreateProject(ctx, w.team.ID, "empty-gap", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.st.DeleteProject(ctx, w.team.ID, extra.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.GetProject(ctx, w.team.ID, extra.ID); err == nil {
		t.Fatal("deleted project still readable")
	}

	tok2, err := w.st.CreateIntegrationToken(ctx, w.team.ID, "doppler", "gh", encrypt(t, w.st, "enc-secret"), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := w.st.ListIntegrationTokens(ctx, w.team.ID)
	if err != nil || len(tokens) != 1 {
		t.Fatal(tokens, err)
	}
	provider, sec, _, err := w.st.IntegrationTokenSecret(ctx, w.team.ID, tok2.ID)
	if err != nil || sec != "enc-secret" || provider != "doppler" {
		t.Fatal(provider, sec, err)
	}
	next := encrypt(t, w.st, "enc2")
	if _, err := w.st.UpdateIntegrationToken(ctx, w.team.ID, tok2.ID, "gh2", &next, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.st.DeleteIntegrationToken(ctx, w.team.ID, tok2.ID); err != nil {
		t.Fatal(err)
	}
	_ = uuid.Nil
}

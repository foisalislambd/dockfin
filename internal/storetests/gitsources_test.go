package storetests

import (
	"errors"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestGitSources(t *testing.T) {
	w := newWorld(t, "git@example.com", "git")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID

	src, err := st.CreateGitSource(ctx, teamID, "github", "acme-app", "acme", "4242", "cid", "client-secret", "private-key", "hook", "https://github.com", "https://api.github.com", "git", 2222)
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "acme-app" || src.CustomPort != 2222 || !src.Configured || src.Installed || src.Applications != 0 {
		t.Fatalf("created %+v", src)
	}
	byID, err := st.GetGitSourceByID(ctx, src.ID)
	if err != nil || byID.TeamID != teamID || byID.AppID != "4242" {
		t.Fatalf("by id %+v %v", byID, err)
	}
	org := "acme-inc"
	renamed, err := st.UpdateGitSource(ctx, teamID, src.ID, store.UpdateGitSourceInput{Organization: &org})
	if err != nil || renamed.Organization != "acme-inc" {
		t.Fatalf("update %+v %v", renamed, err)
	}
	if err := st.UpdateGitSourceInstallation(ctx, teamID, src.ID, "99"); err != nil {
		t.Fatal(err)
	}
	installed, err := st.GetGitSource(ctx, teamID, src.ID)
	if err != nil || !installed.Installed || installed.InstallationID != "99" {
		t.Fatalf("installed %+v %v", installed, err)
	}
	found, secrets, err := st.GetGitSourceByAppID(ctx, "4242")
	if err != nil || found.ID != src.ID || secrets.PrivateKeyEnc != "private-key" || secrets.WebhookSecretEnc != "hook" || secrets.ClientSecretEnc != "client-secret" {
		t.Fatalf("by app %+v %+v %v", found, secrets, err)
	}
	byInst, instSecrets, err := st.GetGitSourceByInstallation(ctx, "4242", "99")
	if err != nil || byInst.ID != src.ID || instSecrets.InstallationID != "99" {
		t.Fatalf("by installation %+v %+v %v", byInst, instSecrets, err)
	}
	if _, _, err := st.GetGitSourceByAppID(ctx, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("empty app id %v", err)
	}
	if _, _, err := st.GetGitSourceByInstallation(ctx, "4242", "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing installation %v", err)
	}

	if err := st.ApplyManifestCredentials(ctx, teamID, src.ID, "5151", "renamed", "new-client", "new-secret", "new-key", "new-hook"); err != nil {
		t.Fatal(err)
	}
	manifest, manifestSecrets, err := st.GetGitSourceByAppID(ctx, "5151")
	if err != nil || manifest.Name != "renamed" || manifest.ClientID != "new-client" || manifestSecrets.PrivateKeyEnc != "new-key" {
		t.Fatalf("manifest %+v %+v %v", manifest, manifestSecrets, err)
	}
	if _, _, err := st.GetGitSourceByAppID(ctx, "4242"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("old app id still resolves: %v", err)
	}

	state := "setup-" + uuid.NewString()
	if err := st.SaveGitSetupState(ctx, state, teamID, src.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	gotTeam, gotSource, err := st.ConsumeGitSetupState(ctx, state)
	if err != nil || gotTeam != teamID || gotSource != src.ID {
		t.Fatalf("consume %s %s %v", gotTeam, gotSource, err)
	}
	if _, _, err := st.ConsumeGitSetupState(ctx, state); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second consume %v", err)
	}
	expired := "expired-" + uuid.NewString()
	if err := st.SaveGitSetupState(ctx, expired, teamID, src.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.ConsumeGitSetupState(ctx, expired); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired consume %v", err)
	}

	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: teamID, EnvironmentID: w.env.ID, GitSourceID: &src.ID, Name: "from-git", BuildPack: "dockerfile",
		GitRepository: "https://github.com/acme/web.git", GitBranch: "main", PortsExposes: "80", BaseDirectory: "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := st.CountAppsUsingGitSource(ctx, teamID, src.ID)
	if err != nil || n != 1 {
		t.Fatalf("count %d %v", n, err)
	}
	refs, err := st.ListAppsUsingGitSource(ctx, teamID, src.ID)
	if err != nil || len(refs) != 1 || refs[0].ID != app.ID || refs[0].Name != "from-git" || refs[0].ProjectID != w.project.ID {
		t.Fatalf("refs %+v %v", refs, err)
	}
	if err := st.DeleteGitSource(ctx, teamID, src.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("delete in use %v", err)
	}
	if err := st.DeleteApplication(ctx, teamID, app.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteGitSource(ctx, teamID, src.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetGitSource(ctx, teamID, src.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("after delete %v", err)
	}
}

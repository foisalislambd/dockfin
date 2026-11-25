package storetests

// CloneEnvironment copies env vars through copyResourceExtras, which does not set
// UpsertEnvVarInput.IsPreview. A preview row with the same key overwrites the
// production value. THEME is therefore not asserted; LITERAL has no preview twin.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestEnvCloneAuthAndAccess(t *testing.T) {
	w := newWorld(t, "owner@example.com", "platform")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID
	app := w.app(t, "web", "dockerfile")
	db := w.database(t, "pg")
	svc := w.service(t, "n8n")

	if _, err := st.UpsertSharedEnv(ctx, teamID, "team", nil, "COLOR", "blue", false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertSharedEnv(ctx, teamID, "project", &w.project.ID, "TIER", "gold", false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertSharedEnv(ctx, teamID, "environment", &w.env.ID, "STAGE", "prod", true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "THEME", Value: "{{team.COLOR}}-{{project.TIER}}", Runtime: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "THEME", Value: "preview-theme", Runtime: true, IsPreview: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "LITERAL", Value: "{{team.COLOR}}", Runtime: true, Literal: true,
	}); err != nil {
		t.Fatal(err)
	}
	resolved, err := st.ResolvedEnvMap(ctx, teamID, "application", app.ID, &w.project.ID, &w.env.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["THEME"] != "blue-gold" || resolved["LITERAL"] != "{{team.COLOR}}" {
		t.Fatalf("resolved %#v", resolved)
	}
	preview, err := st.ResolvedEnvMapPreview(ctx, teamID, "application", app.ID, &w.project.ID, &w.env.ID, nil)
	if err != nil || preview["THEME"] != "preview-theme" || preview["LITERAL"] != "{{team.COLOR}}" {
		t.Fatalf("preview %#v %v", preview, err)
	}
	values := map[string]string{"X": "{{team.COLOR}}", "Y": "{{environment.STAGE}}", "Z": "{{team.COLOR}}"}
	literal := map[string]bool{"Z": true}
	st.ApplySharedRefs(ctx, teamID, &w.project.ID, &w.env.ID, nil, values, literal)
	if values["X"] != "blue" || values["Y"] != "prod" || values["Z"] != "{{team.COLOR}}" {
		t.Fatalf("shared refs %#v", values)
	}

	scripts := []store.DatabaseInitScript{{Filename: "01.sql", Content: "CREATE TABLE t(id int);"}, {Filename: "02.sql", Content: "INSERT INTO t VALUES (1);"}}
	if err := st.SetDatabaseInitScripts(ctx, teamID, db.ID, scripts); err != nil {
		t.Fatal(err)
	}
	gotScripts, err := st.DatabaseInitScripts(ctx, teamID, db.ID)
	if err != nil || len(gotScripts) != 2 || gotScripts[0].Filename != "01.sql" || gotScripts[1].Content != "INSERT INTO t VALUES (1);" {
		t.Fatalf("scripts %+v %v", gotScripts, err)
	}
	other := w.database(t, "pg-copy")
	if err := st.CopyDatabaseInitScripts(ctx, teamID, db.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	copied, err := st.DatabaseInitScripts(ctx, teamID, other.ID)
	if err != nil || len(copied) != 2 || copied[0].Content != scripts[0].Content {
		t.Fatalf("copied %+v %v", copied, err)
	}
	if err := st.SetDatabaseInitScripts(ctx, teamID, other.ID, nil); err != nil {
		t.Fatal(err)
	}
	cleared, err := st.DatabaseInitScripts(ctx, teamID, other.ID)
	if err != nil || len(cleared) != 0 {
		t.Fatalf("cleared %+v %v", cleared, err)
	}

	if _, err := st.CreateScheduledTask(ctx, teamID, "application", app.ID, "nightly", "true", "0 1 * * *", ""); err != nil {
		t.Fatal(err)
	}
	cloned, err := st.CloneEnvironment(ctx, teamID, w.project.ID, w.env.ID, "staging", "stage")
	if err != nil {
		t.Fatal(err)
	}
	if cloned.Applications != 1 || cloned.Databases != 2 || cloned.Services != 1 || cloned.Environment.Name != "staging" || cloned.Environment.ProjectID != w.project.ID {
		t.Fatalf("clone %+v", cloned)
	}
	clonedApps, err := st.ListApplications(ctx, teamID, &cloned.Environment.ID)
	if err != nil || len(clonedApps) != 1 || clonedApps[0].Name != "web-staging" || clonedApps[0].FQDN != "" || clonedApps[0].ID == app.ID {
		t.Fatalf("cloned apps %+v %v", clonedApps, err)
	}
	clonedVars, err := st.ListEnvVars(ctx, teamID, "application", clonedApps[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}
	literalVar := mustFind(t, clonedVars, func(v store.EnvVar) bool { return v.Key == "LITERAL" })
	if literalVar.Value != "{{team.COLOR}}" || !literalVar.IsLiteral || !literalVar.IsRuntime {
		t.Fatalf("cloned literal %+v", literalVar)
	}
	tasks, err := st.ListScheduledTasks(ctx, teamID, "application", &clonedApps[0].ID)
	if err != nil || len(tasks) != 1 || tasks[0].Name != "nightly" || tasks[0].Command != "true" {
		t.Fatalf("cloned tasks %+v %v", tasks, err)
	}
	clonedDBs, err := st.ListDatabases(ctx, teamID, &cloned.Environment.ID)
	if err != nil || len(clonedDBs) != 2 {
		t.Fatalf("cloned dbs %+v %v", clonedDBs, err)
	}
	srcClone := mustFind(t, clonedDBs, func(d store.Database) bool { return d.Name == "pg-staging" })
	init, err := st.DatabaseInitScripts(ctx, teamID, srcClone.ID)
	if err != nil || len(init) != 2 {
		t.Fatalf("cloned init %+v %v", init, err)
	}
	clonedSvcs, err := st.ListServices(ctx, teamID, &cloned.Environment.ID)
	if err != nil || len(clonedSvcs) != 1 || clonedSvcs[0].Name != "n8n-staging" || clonedSvcs[0].FQDN != "" {
		t.Fatalf("cloned services %+v %v", clonedSvcs, err)
	}
	envShared, err := st.ListSharedEnv(ctx, teamID, "environment", &cloned.Environment.ID, true)
	if err != nil || len(envShared) != 1 || envShared[0].Key != "STAGE" || envShared[0].Value != "prod" {
		t.Fatalf("cloned shared %+v %v", envShared, err)
	}
	if _, err := st.CloneEnvironment(ctx, teamID, uuid.New(), w.env.ID, "nope", ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("clone wrong project %v", err)
	}

	if err := st.SetUserTOTPSecret(ctx, w.user.ID, "JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatal(err)
	}
	secret, enabled, err := st.GetUserTOTPSecret(ctx, w.user.ID)
	if err != nil || secret != "JBSWY3DPEHPK3PXP" || enabled {
		t.Fatalf("totp %q %v %v", secret, enabled, err)
	}
	if err := st.EnableUserTOTP(ctx, w.user.ID, []string{"CODE-1", "CODE-2"}); err != nil {
		t.Fatal(err)
	}
	has, err := st.UserHasTOTP(ctx, w.user.ID)
	if err != nil || !has {
		t.Fatalf("totp enabled %v %v", has, err)
	}
	if err := st.SetUserTOTPSecret(ctx, w.user.ID, "NEW"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("overwrite enabled totp %v", err)
	}
	used, err := st.ConsumeRecoveryCode(ctx, w.user.ID, "code-1")
	if err != nil || !used {
		t.Fatalf("consume code %v %v", used, err)
	}
	if used, err := st.ConsumeRecoveryCode(ctx, w.user.ID, "CODE-1"); err != nil || used {
		t.Fatalf("code reused %v %v", used, err)
	}
	challenge, err := st.CreateAuthChallenge(ctx, w.user.ID, "totp", time.Hour)
	if err != nil || challenge.Kind != "totp" || challenge.UserID != w.user.ID {
		t.Fatalf("challenge %+v %v", challenge, err)
	}
	loaded, err := st.GetAuthChallenge(ctx, challenge.ID)
	if err != nil || loaded.ID != challenge.ID || !loaded.ExpiresAt.After(time.Now()) {
		t.Fatalf("get challenge %+v %v", loaded, err)
	}
	consumed, err := st.ConsumeAuthChallenge(ctx, challenge.ID)
	if err != nil || consumed.UserID != w.user.ID {
		t.Fatalf("consume challenge %+v %v", consumed, err)
	}
	if _, err := st.GetAuthChallenge(ctx, challenge.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("challenge still present %v", err)
	}
	if err := st.DisableUserTOTP(ctx, w.user.ID); err != nil {
		t.Fatal(err)
	}
	if has, err := st.UserHasTOTP(ctx, w.user.ID); err != nil || has {
		t.Fatalf("totp still on %v %v", has, err)
	}

	if on, err := st.PasskeysEnabled(ctx); err != nil || !on {
		t.Fatalf("passkeys default %v %v", on, err)
	}
	if err := st.SetPasskeysEnabled(ctx, false); err != nil {
		t.Fatal(err)
	}
	if on, err := st.PasskeysEnabled(ctx); err != nil || on {
		t.Fatalf("passkeys off %v %v", on, err)
	}
	if err := st.SetPasskeysEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}
	credID := []byte("cred-1")
	saved, err := st.SavePasskey(ctx, w.user.ID, "laptop", credID, json.RawMessage(`{"id":"cred-1"}`))
	if err != nil || saved.Name != "laptop" || saved.UserID != w.user.ID {
		t.Fatalf("passkey %+v %v", saved, err)
	}
	if err := st.RenamePasskey(ctx, w.user.ID, saved.ID, "work laptop"); err != nil {
		t.Fatal(err)
	}
	if err := st.TouchPasskey(ctx, saved.ID, json.RawMessage(`{"id":"cred-1","sign":2}`)); err != nil {
		t.Fatal(err)
	}
	found, err := st.PasskeyByCredentialID(ctx, credID)
	if err != nil || found.Name != "work laptop" || found.LastUsedAt == nil {
		t.Fatalf("found passkey %+v %v", found, err)
	}
	var credBody map[string]any
	if err := json.Unmarshal(found.Credential, &credBody); err != nil || credBody["sign"] != float64(2) {
		t.Fatalf("credential %s %v", found.Credential, err)
	}
	list, err := st.ListPasskeys(ctx, w.user.ID)
	if err != nil || len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list %+v %v", list, err)
	}
	sessionUser := w.user.ID
	sessionID, err := st.SaveWebAuthnSession(ctx, &sessionUser, "login", json.RawMessage(`{"challenge":"abc"}`), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	raw, userID, err := st.TakeWebAuthnSession(ctx, sessionID, "login")
	if err != nil || userID == nil || *userID != w.user.ID || !strings.Contains(string(raw), "abc") {
		t.Fatalf("session %s %v %v", raw, userID, err)
	}
	if _, _, err := st.TakeWebAuthnSession(ctx, sessionID, "login"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session reused %v", err)
	}
	if err := st.DeletePasskey(ctx, w.user.ID, saved.ID); err != nil {
		t.Fatal(err)
	}
	list, err = st.ListPasskeys(ctx, w.user.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete %+v %v", list, err)
	}

	invitee, _, err := st.CreateUserWithPersonalTeam(ctx, "member@example.com", "Member", "hash")
	if err != nil {
		t.Fatal(err)
	}
	token := "invite-" + uuid.NewString()
	inv, err := st.CreateInvitation(ctx, teamID, w.user.ID, "Member@Example.com", "member", token, time.Now().Add(24*time.Hour))
	if err != nil || inv.Email != "member@example.com" || inv.Role != "member" {
		t.Fatalf("invite %+v %v", inv, err)
	}
	listed, err := st.ListInvitations(ctx, teamID)
	if err != nil || len(listed) != 1 || listed[0].ID != inv.ID {
		t.Fatalf("invites %+v %v", listed, err)
	}
	previewInv, err := st.PreviewInvitation(ctx, token)
	if err != nil || previewInv.Email != "member@example.com" || previewInv.TeamName != w.team.Name || previewInv.Role != "member" {
		t.Fatalf("preview %+v %v", previewInv, err)
	}
	byToken, err := st.GetInvitationByToken(ctx, token)
	if err != nil || byToken.ID != inv.ID {
		t.Fatalf("by token %+v %v", byToken, err)
	}
	if _, err := st.AcceptInvitation(ctx, token, invitee.ID, "other@example.com"); !errors.Is(err, store.ErrUnauthorized) {
		t.Fatalf("wrong email %v", err)
	}
	joined, err := st.AcceptInvitation(ctx, token, invitee.ID, invitee.Email)
	if err != nil || joined.ID != teamID || joined.Role != "member" {
		t.Fatalf("accept %+v %v", joined, err)
	}
	if _, err := st.GetInvitationByToken(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("invite still valid %v", err)
	}
	members, err := st.ListTeamMembers(ctx, teamID)
	if err != nil {
		t.Fatal(err)
	}
	member := mustFind(t, members, func(m store.TeamMember) bool { return m.UserID == invitee.ID })
	if member.Role != "member" || member.Email != "member@example.com" {
		t.Fatalf("member %+v", member)
	}
	emails, err := st.TeamMemberEmails(ctx, teamID)
	if err != nil || len(emails) != 2 {
		t.Fatalf("emails %#v %v", emails, err)
	}

	if err := st.SetProjectAccess(ctx, teamID, w.user.ID, "projects", nil); err == nil || !strings.Contains(err.Error(), "owners") {
		t.Fatalf("owner access %v", err)
	}
	if err := st.SetProjectAccess(ctx, teamID, invitee.ID, "projects", []store.ProjectGrant{{ProjectID: w.project.ID, Permission: "view"}}); err != nil {
		t.Fatal(err)
	}
	grants, err := st.ListProjectGrants(ctx, teamID, invitee.ID)
	if err != nil || len(grants) != 1 || grants[0].Permission != "view" || grants[0].ProjectID != w.project.ID {
		t.Fatalf("grants %+v %v", grants, err)
	}
	access, err := st.MemberAccess(ctx, teamID, invitee.ID, "member")
	if err != nil || access.Unrestricted || !access.Allows(w.project.ID, false, false) || access.Allows(w.project.ID, true, false) {
		t.Fatalf("access %+v %v", access, err)
	}
	if access.EnvProject[w.env.ID] != w.project.ID {
		t.Fatalf("env map %#v", access.EnvProject)
	}
	ownerAccess, err := st.MemberAccess(ctx, teamID, w.user.ID, "owner")
	if err != nil || !ownerAccess.Unrestricted || !ownerAccess.Allows(w.project.ID, true, true) {
		t.Fatalf("owner access %+v %v", ownerAccess, err)
	}
	projectID, ok, err := st.ProjectIDForResource(ctx, teamID, "application", app.ID)
	if err != nil || !ok || projectID != w.project.ID {
		t.Fatalf("project for app %s %v %v", projectID, ok, err)
	}
	projectID, ok, err = st.ProjectIDForResource(ctx, teamID, "database", db.ID)
	if err != nil || !ok || projectID != w.project.ID {
		t.Fatalf("project for db %s %v %v", projectID, ok, err)
	}
	projectID, ok, err = st.ProjectIDForResource(ctx, teamID, "service", svc.ID)
	if err != nil || !ok || projectID != w.project.ID {
		t.Fatalf("project for service %s %v %v", projectID, ok, err)
	}
	if err := st.SetProjectAccess(ctx, teamID, invitee.ID, "projects", []store.ProjectGrant{{ProjectID: w.project.ID, Permission: "own"}}); err == nil {
		t.Fatal("bad permission accepted")
	}
	if err := st.SetProjectAccess(ctx, teamID, invitee.ID, "all", nil); err != nil {
		t.Fatal(err)
	}
	access, err = st.MemberAccess(ctx, teamID, invitee.ID, "member")
	if err != nil || !access.Unrestricted {
		t.Fatalf("all access %+v %v", access, err)
	}
	grants, err = st.ListProjectGrants(ctx, teamID, invitee.ID)
	if err != nil || len(grants) != 0 {
		t.Fatalf("grants after clear %+v %v", grants, err)
	}

	dropToken := "drop-" + uuid.NewString()
	drop, err := st.CreateInvitation(ctx, teamID, w.user.ID, "gone@example.com", "admin", dropToken, time.Now().Add(time.Hour))
	if err != nil || drop.Role != "admin" {
		t.Fatalf("drop invite %+v %v", drop, err)
	}
	if err := st.DeleteInvitation(ctx, teamID, drop.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PreviewInvitation(ctx, dropToken); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted invite %v", err)
	}

	shared, err := st.CreateTeam(ctx, w.user.ID, "Shared", "company")
	if err != nil || shared.Personal {
		t.Fatalf("shared team %+v %v", shared, err)
	}
	oauthUser, oauthTeam, err := st.CreateUserOAuth(ctx, "oauth@example.com", "Oauth")
	if err != nil || !oauthTeam.Personal || oauthTeam.Role != "owner" || oauthUser.Email != "oauth@example.com" {
		t.Fatalf("oauth user %+v %+v %v", oauthUser, oauthTeam, err)
	}
	if err := st.AddUserToOldestNonPersonalTeam(ctx, oauthUser.ID); err != nil {
		t.Fatal(err)
	}
	sharedMembers, err := st.ListTeamMembers(ctx, shared.ID)
	if err != nil {
		t.Fatal(err)
	}
	joinedMember := mustFind(t, sharedMembers, func(m store.TeamMember) bool { return m.UserID == oauthUser.ID })
	if joinedMember.Role != "member" {
		t.Fatalf("auto join %+v", joinedMember)
	}

	uid := w.user.ID
	if err := st.InsertAuditLog(ctx, store.AuditLog{
		TeamID: teamID, UserID: &uid, ActorEmail: "owner@example.com", Method: "POST", Path: "/api/v1/projects",
		Action: "project.create", Category: "mutation", Summary: "created platform", ResourceType: "project",
		ResourceID: w.project.ID.String(), StatusCode: 201, IP: "127.0.0.1", UserAgent: "test",
	}); err != nil {
		t.Fatal(err)
	}
	logs, err := st.ListAuditLogs(ctx, teamID, 10)
	if err != nil || len(logs) != 1 || logs[0].Action != "project.create" || logs[0].UserEmail != "owner@example.com" || logs[0].StatusCode != 201 {
		t.Fatalf("audit %+v %v", logs, err)
	}
	page, err := st.QueryAuditLogs(ctx, store.AuditQuery{TeamID: teamID, Action: "project.create", Query: "platform", Limit: 5})
	if err != nil || page.Total != 1 || len(page.Logs) != 1 || page.Logs[0].Summary != "created platform" {
		t.Fatalf("query %+v %v", page, err)
	}
	if n, err := st.PruneAuditLogs(ctx, 0); err != nil || n != 0 {
		t.Fatalf("prune forever %d %v", n, err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE audit_logs SET created_at = NOW() - INTERVAL '10 days' WHERE team_id=$1`, teamID); err != nil {
		t.Fatal(err)
	}
	n, err := st.PruneAuditLogs(ctx, 1)
	if err != nil || n < 1 {
		t.Fatalf("prune %d %v", n, err)
	}
	logs, err = st.ListAuditLogs(ctx, teamID, 10)
	if err != nil || len(logs) != 0 {
		t.Fatalf("logs after prune %+v %v", logs, err)
	}
}

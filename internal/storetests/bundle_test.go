package storetests

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func TestSecretBundleRoundTrip(t *testing.T) {
	w := newWorld(t, "bundle@example.com", "bundle")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID

	pemText, authorized, fingerprint := testSSHKey(t)
	key, err := st.CreatePrivateKey(ctx, teamID, "deploy", "ci", authorized, encrypt(t, st, pemText), fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := st.CreateServer(ctx, teamID, &key.ID, "edge", "edge box", "203.0.113.10", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	dest, err := st.CreateDestination(ctx, teamID, srv.ID, "Apps", "standalone", "apps")
	if err != nil {
		t.Fatal(err)
	}

	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: teamID, EnvironmentID: w.env.ID, DestinationID: &dest.ID, Name: "web", BuildPack: "dockerfile",
		Dockerfile: "FROM nginx:alpine\n", GitRepository: "https://github.com/acme/web.git", GitBranch: "main",
		PortsExposes: "80", BaseDirectory: "/", FQDN: "web.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetWebhookSecret(ctx, teamID, app.ID, "hook-secret"); err != nil {
		t.Fatal(err)
	}
	portVar, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "PORT", Value: "8080", Runtime: true, Buildtime: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetEnvVarLocked(ctx, teamID, portVar.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "GREETING", Value: "{{team.SHARED}}", Runtime: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{
		Key: "APP_REGION", Value: "{{server.REGION}}", Runtime: true,
	}); err != nil {
		t.Fatal(err)
	}

	cred, err := st.Box.EncryptString("db-secret")
	if err != nil {
		t.Fatal(err)
	}
	db, err := st.CreateDatabase(ctx, &store.Database{
		TeamID: teamID, EnvironmentID: w.env.ID, DestinationID: &dest.ID,
		Name: "pg", Engine: "postgresql",
	}, cred)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "database", db.ID, store.UpsertEnvVarInput{
		Key: "POSTGRES_DB", Value: "app", Runtime: true, Literal: true,
	}); err != nil {
		t.Fatal(err)
	}

	sid := srv.ID
	svc, err := st.CreateService(ctx, &store.Service{
		TeamID: teamID, EnvironmentID: w.env.ID, ServerID: &sid, DestinationID: &dest.ID,
		Name: "n8n", ServiceType: "n8n", FQDN: "n8n.example.com",
		DockerComposeRaw: "services:\n  n8n:\n    image: n8nio/n8n\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "service", svc.ID, store.UpsertEnvVarInput{
		Key: "N8N_PORT", Value: "5678", Runtime: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertSharedEnv(ctx, teamID, "team", nil, "SHARED", "team-value", false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertSharedEnv(ctx, teamID, "server", &srv.ID, "REGION", "eu", false); err != nil {
		t.Fatal(err)
	}

	if _, err := st.ExportSecretBundle(ctx, teamID, uuid.New(), "passphrase-bundle-test"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing server: %v", err)
	}
	envelope, err := st.ExportSecretBundle(ctx, teamID, srv.ID, "passphrase-bundle-test")
	if err != nil {
		t.Fatal(err)
	}
	if envelope == "" {
		t.Fatal("empty envelope")
	}
	if _, err := st.ImportSecretBundle(ctx, teamID, "wrong-passphrase", envelope); err == nil {
		t.Fatal("wrong passphrase was accepted")
	}

	_, otherTeam, err := st.CreateUserWithPersonalTeam(ctx, "import@example.com", "Importer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	res, err := st.ImportSecretBundle(ctx, otherTeam.ID, "passphrase-bundle-test", envelope)
	if err != nil {
		t.Fatal(err)
	}
	if res.Projects != 1 || res.Apps != 1 || res.Databases != 1 || res.Services != 1 {
		t.Fatalf("import counts %+v", res)
	}
	importedSrv, err := st.GetServer(ctx, otherTeam.ID, res.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	if importedSrv.Name != "edge" || importedSrv.IP != "203.0.113.10" || importedSrv.Port != 22 || importedSrv.UserName != "root" {
		t.Fatalf("imported server %+v", importedSrv)
	}
	keys, err := st.ListPrivateKeys(ctx, otherTeam.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Fingerprint != fingerprint {
		t.Fatalf("imported key %+v", keys)
	}

	apps, err := st.ListApplications(ctx, otherTeam.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].Name != "web" || apps[0].FQDN != "web.example.com" || apps[0].GitBranch != "main" {
		t.Fatalf("imported apps %+v", apps)
	}
	hook, err := st.GetWebhookSecret(ctx, apps[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if hook != "hook-secret" {
		t.Fatalf("webhook %q", hook)
	}
	vars, err := st.ListEnvVars(ctx, otherTeam.ID, "application", apps[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]store.EnvVar{}
	for _, v := range vars {
		byKey[v.Key] = v
	}
	if byKey["PORT"].Value != "8080" || !byKey["PORT"].IsLocked || !byKey["PORT"].IsBuildtime {
		t.Fatalf("PORT %+v", byKey["PORT"])
	}
	if byKey["GREETING"].Value != "{{team.SHARED}}" {
		t.Fatalf("GREETING %+v", byKey["GREETING"])
	}
	resolved, err := st.ResolvedEnvMap(ctx, otherTeam.ID, "application", apps[0].ID, nil, nil, &importedSrv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["GREETING"] != "team-value" || resolved["APP_REGION"] != "eu" || resolved["PORT"] != "8080" {
		t.Fatalf("resolved %#v", resolved)
	}

	dbs, err := st.ListDatabases(ctx, otherTeam.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbs) != 1 || dbs[0].Name != "pg" || dbs[0].Engine != "postgresql" {
		t.Fatalf("imported dbs %+v", dbs)
	}
	encCred, err := st.GetDatabaseCredentials(ctx, otherTeam.ID, dbs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	plainCred, err := st.Box.DecryptString(encCred)
	if err != nil {
		t.Fatal(err)
	}
	if plainCred != "db-secret" {
		t.Fatalf("credentials %q", plainCred)
	}
	dbVars, err := st.ListEnvVars(ctx, otherTeam.ID, "database", dbs[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbVars) != 1 || dbVars[0].Key != "POSTGRES_DB" || dbVars[0].Value != "app" || !dbVars[0].IsLiteral {
		t.Fatalf("db vars %+v", dbVars)
	}

	svcs, err := st.ListServices(ctx, otherTeam.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 1 || svcs[0].Name != "n8n" || svcs[0].FQDN != "n8n.example.com" || svcs[0].ServerID == nil || *svcs[0].ServerID != importedSrv.ID {
		t.Fatalf("imported services %+v", svcs)
	}
	svcVars, err := st.ListEnvVars(ctx, otherTeam.ID, "service", svcs[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcVars) != 1 || svcVars[0].Value != "5678" {
		t.Fatalf("service vars %+v", svcVars)
	}
	shared, err := st.ListSharedEnv(ctx, otherTeam.ID, "team", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(shared) != 1 || shared[0].Key != "SHARED" || shared[0].Value != "team-value" {
		t.Fatalf("shared %+v", shared)
	}
	serverShared, err := st.ListSharedEnv(ctx, otherTeam.ID, "server", &importedSrv.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(serverShared) != 1 || serverShared[0].Key != "REGION" || serverShared[0].Value != "eu" {
		t.Fatalf("server shared %+v", serverShared)
	}
	dests, err := st.ListDestinations(ctx, otherTeam.ID, &importedSrv.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundAppsNet := false
	for _, d := range dests {
		if d.Network == "apps" && d.Name == "Apps" {
			foundAppsNet = true
		}
	}
	if !foundAppsNet {
		t.Fatalf("destinations %+v", dests)
	}
}

func testSSHKey(t *testing.T) (pemText, authorized, fingerprint string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "dockfin-test")
	if err != nil {
		t.Fatal(err)
	}
	pemText = string(pem.EncodeToMemory(block))
	signer, err := ssh.ParsePrivateKey([]byte(pemText))
	if err != nil {
		t.Fatal(err)
	}
	return pemText, string(ssh.MarshalAuthorizedKey(signer.PublicKey())), sshx.FingerprintSHA256(signer.PublicKey())
}

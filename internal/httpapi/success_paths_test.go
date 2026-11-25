package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestSuccessPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	st := testkit.OpenStore(t)
	api := &API{
		Cfg: &config.Config{
			Env: "development", PublicURL: "http://127.0.0.1:8000",
			MasterKey: "test-master-key-for-integration-32b!", SessionTTL: time.Hour,
			RateLimitPerMinute: 0, AuthRateLimitPer15Min: 0, WebhookRateLimitPerMinute: 0,
			AuditRetentionDays: 90,
		},
		Store:  st,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(api.Router())
	t.Cleanup(srv.Close)

	ownerToken := registerOwner(t, srv.Client(), srv.URL)
	ctx := context.Background()
	owner := userByEmail(t, st, "owner@example.com")
	teamID := mustTeam(t, st, "owner@example.com")

	call := func(token, method, path string, body any) (int, map[string]any, string) {
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
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		return res.StatusCode, parsed, string(raw)
	}
	do := func(method, path string, body any) (int, map[string]any, string) {
		t.Helper()
		return call(ownerToken, method, path, body)
	}
	requireStatus := func(what string, code int, want int, parsed map[string]any, raw string) {
		t.Helper()
		if code != want {
			t.Fatalf("%s: status %d body %s", what, code, raw)
		}
	}
	field := func(m map[string]any, keys ...string) string {
		t.Helper()
		var cur any = m
		for _, k := range keys {
			mm, ok := cur.(map[string]any)
			if !ok {
				return ""
			}
			cur = mm[k]
		}
		s, _ := cur.(string)
		return s
	}

	code, reg, raw := call("", http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email": "second@example.com", "name": "Second", "password": "password12345",
	})
	requireStatus("second register", code, http.StatusForbidden, reg, raw)
	if !strings.Contains(strings.ToLower(field(reg, "error")), "registration") {
		t.Fatalf("registration lock error: %s", raw)
	}
	code, regStatus, raw := call("", http.MethodGet, "/api/v1/auth/registration", nil)
	requireStatus("registration status", code, http.StatusOK, regStatus, raw)
	if regStatus["registration_enabled"] != false {
		t.Fatalf("registration still enabled: %s", raw)
	}

	code, settings, raw := do(http.MethodGet, "/api/v1/settings", nil)
	requireStatus("settings get", code, http.StatusOK, settings, raw)
	if _, ok := settings["settings"].(map[string]any); !ok {
		t.Fatalf("settings payload: %s", raw)
	}
	code, patchedSettings, raw := do(http.MethodPatch, "/api/v1/settings", map[string]any{
		"instance_name":             "Dockfin Lab",
		"is_dns_validation_enabled": false,
	})
	requireStatus("settings patch", code, http.StatusOK, patchedSettings, raw)
	if field(patchedSettings, "settings", "instance_name") != "Dockfin Lab" {
		t.Fatalf("instance name: %s", raw)
	}
	if patchedSettings["settings"].(map[string]any)["is_dns_validation_enabled"] != false {
		t.Fatalf("dns validation flag: %s", raw)
	}

	code, oauth, raw := do(http.MethodGet, "/api/v1/settings/oauth", nil)
	requireStatus("oauth settings", code, http.StatusOK, oauth, raw)
	if _, ok := oauth["oauth_settings"].([]any); !ok {
		t.Fatalf("oauth settings: %s", raw)
	}

	code, created, raw := do(http.MethodPost, "/api/v1/projects", map[string]string{"name": "visible-proj", "description": "shown"})
	requireStatus("create project", code, http.StatusCreated, created, raw)
	projectID := field(created, "project", "id")
	envID := field(created, "environment", "id")
	if projectID == "" || envID == "" {
		t.Fatalf("project ids: %s", raw)
	}
	code, updatedProj, raw := do(http.MethodPatch, "/api/v1/projects/"+projectID, map[string]string{
		"name": "visible-renamed", "description": "updated",
	})
	requireStatus("update project", code, http.StatusOK, updatedProj, raw)
	if field(updatedProj, "name") != "visible-renamed" {
		t.Fatalf("project name: %s", raw)
	}
	code, staging, raw := do(http.MethodPost, "/api/v1/projects/"+projectID+"/environments", map[string]string{
		"name": "staging", "description": "pre",
	})
	requireStatus("create environment", code, http.StatusCreated, staging, raw)
	stagingID := field(staging, "id")
	if stagingID == "" || field(staging, "name") != "staging" {
		t.Fatalf("environment: %s", raw)
	}
	code, frozen, raw := do(http.MethodPut, "/api/v1/projects/"+projectID+"/environments/"+stagingID+"/freeze", map[string]any{
		"frozen": true, "reason": "release window",
	})
	requireStatus("freeze", code, http.StatusOK, frozen, raw)
	if frozen["deploy_frozen"] != true || field(frozen, "deploy_freeze_reason") != "release window" {
		t.Fatalf("freeze body: %s", raw)
	}
	code, thawed, raw := do(http.MethodPut, "/api/v1/projects/"+projectID+"/environments/"+stagingID+"/freeze", map[string]any{
		"frozen": false, "reason": "",
	})
	requireStatus("unfreeze", code, http.StatusOK, thawed, raw)
	if thawed["deploy_frozen"] != false {
		t.Fatalf("unfreeze body: %s", raw)
	}
	code, envs, raw := do(http.MethodGet, "/api/v1/projects/"+projectID+"/environments", nil)
	requireStatus("list environments", code, http.StatusOK, envs, raw)
	if !strings.Contains(raw, "staging") {
		t.Fatalf("environments missing staging: %s", raw)
	}

	code, app, raw := do(http.MethodPost, "/api/v1/applications", map[string]string{
		"environment_id": envID, "name": "pulse-web", "build_pack": "dockerfile", "dockerfile": "FROM nginx:alpine\n",
	})
	requireStatus("create application", code, http.StatusCreated, app, raw)
	appID := field(app, "id")
	if appID == "" || field(app, "name") != "pulse-web" {
		t.Fatalf("application: %s", raw)
	}

	code, fakeSrv, raw := do(http.MethodPost, "/api/v1/servers", map[string]any{
		"name": "fake-edge", "ip": "999.999.999.999", "port": 22, "user_name": "root", "proxy_type": "traefik",
	})
	if code == http.StatusBadRequest {
		if strings.TrimSpace(field(fakeSrv, "error")) == "" {
			t.Fatalf("fake server 400 missing error text: %s", raw)
		}
	} else {
		requireStatus("create server fake ip", code, http.StatusCreated, fakeSrv, raw)
		if field(fakeSrv, "name") != "fake-edge" {
			t.Fatalf("fake server: %s", raw)
		}
	}
	code, edge, raw := do(http.MethodPost, "/api/v1/servers", map[string]any{
		"name": "edge", "description": "lab", "ip": "203.0.113.10", "port": 22, "user_name": "root", "proxy_type": "traefik",
	})
	requireStatus("create server", code, http.StatusCreated, edge, raw)
	serverID := field(edge, "id")
	if serverID == "" || field(edge, "ip") != "203.0.113.10" {
		t.Fatalf("server: %s", raw)
	}
	code, gotSrv, raw := do(http.MethodGet, "/api/v1/servers/"+serverID, nil)
	requireStatus("get server", code, http.StatusOK, gotSrv, raw)
	if field(gotSrv, "name") != "edge" {
		t.Fatalf("get server: %s", raw)
	}
	code, exported, raw := do(http.MethodGet, "/api/v1/servers/"+serverID+"/export", nil)
	requireStatus("export server", code, http.StatusOK, exported, raw)
	if field(exported, "name") != "edge" || exported["version"] == nil {
		t.Fatalf("export: %s", raw)
	}
	code, incomplete, raw := do(http.MethodPost, "/api/v1/server-imports", map[string]any{
		"server": map[string]any{"version": 1, "name": "partial"},
	})
	requireStatus("import incomplete", code, http.StatusBadRequest, incomplete, raw)
	if !strings.Contains(strings.ToLower(field(incomplete, "error")), "incomplete") {
		t.Fatalf("import error: %s", raw)
	}
	code, dests, raw := do(http.MethodGet, "/api/v1/destinations?server_id="+serverID, nil)
	requireStatus("destinations", code, http.StatusOK, dests, raw)
	if !strings.Contains(raw, "Default") {
		t.Fatalf("destinations: %s", raw)
	}
	code, generated, raw := do(http.MethodPost, "/api/v1/domains/generate", map[string]string{
		"name": "pulse-web", "server_id": serverID, "resource_id": appID,
	})
	requireStatus("generate domain", code, http.StatusOK, generated, raw)
	fqdn := field(generated, "fqdn")
	if fqdn == "" || field(generated, "url") == "" {
		t.Fatalf("generated domain: %s", raw)
	}
	code, dnsChecked, raw := do(http.MethodPost, "/api/v1/domains/check", map[string]string{
		"domains": fqdn, "server_id": serverID, "expected_ip": "203.0.113.10",
	})
	requireStatus("check domain", code, http.StatusOK, dnsChecked, raw)
	if _, ok := dnsChecked["results"].([]any); !ok || dnsChecked["expected_ip"] != "203.0.113.10" {
		t.Fatalf("dns check: %s", raw)
	}
	code, patchedApp, raw := do(http.MethodPatch, "/api/v1/applications/"+appID, map[string]any{"fqdn": fqdn})
	requireStatus("patch fqdn", code, http.StatusOK, patchedApp, raw)
	if field(patchedApp, "fqdn") != fqdn && !strings.Contains(raw, fqdn) {
		t.Fatalf("patched fqdn: %s", raw)
	}
	code, gotApp, raw := do(http.MethodGet, "/api/v1/applications/"+appID, nil)
	requireStatus("get application", code, http.StatusOK, gotApp, raw)
	if !strings.Contains(field(gotApp, "fqdn"), strings.Split(fqdn, ",")[0]) && !strings.Contains(raw, "pulse-web") {
		t.Fatalf("application fqdn: %s", raw)
	}

	code, ev, raw := do(http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": appID, "key": "PORT", "value": "80", "is_runtime": true,
	})
	requireStatus("create env", code, http.StatusOK, ev, raw)
	varID := field(ev, "id")
	if varID == "" || field(ev, "key") != "PORT" {
		t.Fatalf("env var: %s", raw)
	}
	code, ev, raw = do(http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": appID, "key": "PORT", "value": "8080", "is_runtime": true,
	})
	requireStatus("update env", code, http.StatusOK, ev, raw)
	if field(ev, "key") != "PORT" || field(ev, "value") != "8080" {
		t.Fatalf("updated env: %s", raw)
	}
	code, listedEnv, raw := do(http.MethodGet, "/api/v1/env-vars?resource_type=application&resource_id="+appID, nil)
	requireStatus("list env", code, http.StatusOK, listedEnv, raw)
	if !strings.Contains(raw, "PORT") {
		t.Fatalf("list env: %s", raw)
	}
	locked := true
	code, ev, raw = do(http.MethodPost, "/api/v1/env-vars/"+varID+"/lock", map[string]any{"locked": locked})
	requireStatus("lock env", code, http.StatusOK, ev, raw)
	if ev["is_locked"] != true {
		t.Fatalf("lock env: %s", raw)
	}
	locked = false
	code, _, raw = do(http.MethodPost, "/api/v1/env-vars/"+varID+"/lock", map[string]any{"locked": locked})
	requireStatus("unlock env", code, http.StatusOK, nil, raw)
	code, deletedEnv, raw := do(http.MethodDelete, "/api/v1/env-vars/"+varID, nil)
	requireStatus("delete env", code, http.StatusOK, deletedEnv, raw)
	if field(deletedEnv, "status") != "deleted" {
		t.Fatalf("delete env: %s", raw)
	}
	code, shared, raw := do(http.MethodPost, "/api/v1/shared-env-vars", map[string]any{
		"scope_type": "team", "key": "SHARED_REGION", "value": "lab", "is_literal": true,
	})
	requireStatus("shared env", code, http.StatusOK, shared, raw)
	if field(shared, "key") != "SHARED_REGION" {
		t.Fatalf("shared env: %s", raw)
	}
	code, sharedList, raw := do(http.MethodGet, "/api/v1/shared-env-vars?scope_type=team", nil)
	requireStatus("list shared env", code, http.StatusOK, sharedList, raw)
	if !strings.Contains(raw, "SHARED_REGION") {
		t.Fatalf("shared list: %s", raw)
	}

	code, meta, raw := do(http.MethodPost, "/api/v1/s3-storages", map[string]any{
		"name": "metadata", "endpoint": "http://169.254.169.254", "bucket": "bucket", "region": "us-east-1",
		"access_key": "ak", "secret_key": "sk",
	})
	requireStatus("s3 metadata", code, http.StatusBadRequest, meta, raw)
	if !strings.Contains(strings.ToLower(field(meta, "error")), "not an allowed") {
		t.Fatalf("metadata rejection: %s", raw)
	}
	code, s3, raw := do(http.MethodPost, "/api/v1/s3-storages", map[string]any{
		"name": "b2", "endpoint": "https://s3.example.com", "bucket": "bucket", "region": "us-east-1",
		"access_key": "ak", "secret_key": "sk",
	})
	requireStatus("create s3", code, http.StatusCreated, s3, raw)
	s3ID := field(s3, "id")
	if s3ID == "" || field(s3, "endpoint") == "" || field(s3, "bucket") != "bucket" {
		t.Fatalf("s3: %s", raw)
	}
	code, s3List, raw := do(http.MethodGet, "/api/v1/s3-storages", nil)
	requireStatus("list s3", code, http.StatusOK, s3List, raw)
	if !strings.Contains(raw, "b2") {
		t.Fatalf("s3 list: %s", raw)
	}
	code, s3Got, raw := do(http.MethodGet, "/api/v1/s3-storages/"+s3ID, nil)
	requireStatus("get s3", code, http.StatusOK, s3Got, raw)
	if field(s3Got, "name") != "b2" {
		t.Fatalf("get s3: %s", raw)
	}

	code, backup, raw := do(http.MethodPost, "/api/v1/scheduled-backups", map[string]any{
		"resource_type": "application", "resource_id": appID, "s3_storage_id": s3ID,
		"frequency": "0 0 * * *", "retention": 3,
	})
	requireStatus("create scheduled backup", code, http.StatusCreated, backup, raw)
	backupID := field(backup, "id")
	if backupID == "" || field(backup, "frequency") != "0 0 * * *" {
		t.Fatalf("backup: %s", raw)
	}
	code, backups, raw := do(http.MethodGet, "/api/v1/scheduled-backups", nil)
	requireStatus("list scheduled backups", code, http.StatusOK, backups, raw)
	if !strings.Contains(raw, backupID) {
		t.Fatalf("backup list: %s", raw)
	}
	freq := "0 3 * * *"
	enabled := false
	code, backup, raw = do(http.MethodPatch, "/api/v1/scheduled-backups/"+backupID, map[string]any{
		"frequency": freq, "enabled": enabled,
	})
	requireStatus("patch scheduled backup", code, http.StatusOK, backup, raw)
	if field(backup, "frequency") != freq || backup["enabled"] != false {
		t.Fatalf("patched backup: %s", raw)
	}
	code, delBackup, raw := do(http.MethodDelete, "/api/v1/scheduled-backups/"+backupID, nil)
	requireStatus("delete scheduled backup", code, http.StatusOK, delBackup, raw)
	if field(delBackup, "status") != "deleted" {
		t.Fatalf("delete backup: %s", raw)
	}

	code, task, raw := do(http.MethodPost, "/api/v1/scheduled-tasks", map[string]any{
		"resource_type": "application", "resource_id": appID, "name": "nightly",
		"command": "echo ok", "frequency": "0 1 * * *", "container_name": "web",
	})
	requireStatus("create scheduled task", code, http.StatusCreated, task, raw)
	taskID := field(task, "id")
	if taskID == "" || field(task, "name") != "nightly" || field(task, "command") != "echo ok" {
		t.Fatalf("task: %s", raw)
	}
	code, tasks, raw := do(http.MethodGet, "/api/v1/scheduled-tasks?resource_type=application&resource_id="+appID, nil)
	requireStatus("list scheduled tasks", code, http.StatusOK, tasks, raw)
	if !strings.Contains(raw, "nightly") {
		t.Fatalf("task list: %s", raw)
	}
	newName := "nightly-renamed"
	code, task, raw = do(http.MethodPatch, "/api/v1/scheduled-tasks/"+taskID, map[string]any{"name": newName})
	requireStatus("patch scheduled task", code, http.StatusOK, task, raw)
	if field(task, "name") != newName {
		t.Fatalf("patched task: %s", raw)
	}
	code, execs, raw := do(http.MethodGet, "/api/v1/scheduled-tasks/"+taskID+"/executions", nil)
	requireStatus("task executions", code, http.StatusOK, execs, raw)
	if _, ok := execs["executions"].([]any); !ok {
		t.Fatalf("executions: %s", raw)
	}
	code, delTask, raw := do(http.MethodDelete, "/api/v1/scheduled-tasks/"+taskID, nil)
	requireStatus("delete scheduled task", code, http.StatusOK, delTask, raw)
	if field(delTask, "status") != "deleted" {
		t.Fatalf("delete task: %s", raw)
	}
	code, delS3, raw := do(http.MethodDelete, "/api/v1/s3-storages/"+s3ID, nil)
	requireStatus("delete s3", code, http.StatusOK, delS3, raw)
	if field(delS3, "status") != "deleted" {
		t.Fatalf("delete s3: %s", raw)
	}

	code, tag, raw := do(http.MethodPost, "/api/v1/tags", map[string]string{"name": "prod", "color": "#112233"})
	requireStatus("create tag", code, http.StatusCreated, tag, raw)
	tagID := field(tag, "id")
	if tagID == "" || field(tag, "name") != "prod" {
		t.Fatalf("tag: %s", raw)
	}
	code, tags, raw := do(http.MethodGet, "/api/v1/tags", nil)
	requireStatus("list tags", code, http.StatusOK, tags, raw)
	if !strings.Contains(raw, "prod") {
		t.Fatalf("tags: %s", raw)
	}
	code, attached, raw := do(http.MethodPost, "/api/v1/tags/attach", map[string]string{
		"tag_id": tagID, "resource_type": "application", "resource_id": appID,
	})
	requireStatus("attach tag", code, http.StatusOK, attached, raw)
	if !strings.Contains(raw, "prod") {
		t.Fatalf("attach: %s", raw)
	}
	code, resourceTags, raw := do(http.MethodGet, "/api/v1/resource-tags?resource_type=application&resource_id="+appID, nil)
	requireStatus("resource tags", code, http.StatusOK, resourceTags, raw)
	if !strings.Contains(raw, tagID) && !strings.Contains(raw, "prod") {
		t.Fatalf("resource tags: %s", raw)
	}
	code, detached, raw := do(http.MethodDelete, "/api/v1/tags/"+tagID+"/attach?resource_type=application&resource_id="+appID, nil)
	requireStatus("detach tag", code, http.StatusOK, detached, raw)
	if field(detached, "status") != "detached" {
		t.Fatalf("detach: %s", raw)
	}
	code, delTag, raw := do(http.MethodDelete, "/api/v1/tags/"+tagID, nil)
	requireStatus("delete tag", code, http.StatusOK, delTag, raw)
	if field(delTag, "status") != "deleted" {
		t.Fatalf("delete tag: %s", raw)
	}

	code, key, raw := do(http.MethodPost, "/api/v1/private-keys/generate", map[string]string{"name": "ci", "type": "ed25519"})
	requireStatus("generate key", code, http.StatusCreated, key, raw)
	if field(key, "fingerprint") == "" && !strings.Contains(field(key, "public_key"), "ssh-ed25519") {
		t.Fatalf("generated key missing fingerprint/public key: %s", raw)
	}
	if !strings.HasPrefix(field(key, "fingerprint"), "SHA256:") && !strings.Contains(field(key, "public_key"), "ssh-ed25519") {
		t.Fatalf("key material: %s", raw)
	}
	keyID := field(key, "id")
	code, keys, raw := do(http.MethodGet, "/api/v1/private-keys", nil)
	requireStatus("list keys", code, http.StatusOK, keys, raw)
	if !strings.Contains(raw, "ci") {
		t.Fatalf("keys: %s", raw)
	}
	code, gotKey, raw := do(http.MethodGet, "/api/v1/private-keys/"+keyID, nil)
	requireStatus("get key", code, http.StatusOK, gotKey, raw)
	if field(gotKey, "name") != "ci" {
		t.Fatalf("get key: %s", raw)
	}

	code, gs, raw := do(http.MethodPost, "/api/v1/git-sources", map[string]any{
		"name": "github-lab", "provider": "github", "html_url": "https://github.com",
	})
	requireStatus("create git source", code, http.StatusCreated, gs, raw)
	if field(gs, "name") != "github-lab" || !strings.Contains(field(gs, "html_url"), "github.com") {
		t.Fatalf("git source: %s", raw)
	}
	code, sources, raw := do(http.MethodGet, "/api/v1/git-sources", nil)
	requireStatus("list git sources", code, http.StatusOK, sources, raw)
	if !strings.Contains(raw, "github-lab") {
		t.Fatalf("git sources: %s", raw)
	}

	code, inv, raw := do(http.MethodPost, "/api/v1/team/invitations", map[string]string{
		"email": "new@example.com", "role": "member",
	})
	requireStatus("create invitation", code, http.StatusCreated, inv, raw)
	if field(inv, "token") == "" || !strings.Contains(field(inv, "invite_url"), "token=") {
		t.Fatalf("invitation: %s", raw)
	}
	code, invites, raw := do(http.MethodGet, "/api/v1/team/invitations", nil)
	requireStatus("list invitations", code, http.StatusOK, invites, raw)
	if !strings.Contains(raw, "new@example.com") {
		t.Fatalf("invitations: %s", raw)
	}

	code, notes, raw := do(http.MethodGet, "/api/v1/notifications", nil)
	requireStatus("list notifications", code, http.StatusOK, notes, raw)
	if !strings.Contains(raw, "email") {
		t.Fatalf("notifications: %s", raw)
	}
	code, note, raw := do(http.MethodPut, "/api/v1/notifications/webhook", map[string]any{
		"enabled": true,
		"events":  []string{"deploy_success"},
		"config":  map[string]any{"url": "https://hooks.example.com/dockfin", "headers": map[string]any{}},
	})
	requireStatus("upsert notification", code, http.StatusOK, note, raw)
	if field(note, "notification", "channel") != "webhook" || note["notification"].(map[string]any)["enabled"] != true {
		t.Fatalf("notification: %s", raw)
	}

	code, pkOn, raw := do(http.MethodGet, "/api/v1/settings/passkeys", nil)
	requireStatus("passkeys get", code, http.StatusOK, pkOn, raw)
	if _, ok := pkOn["enabled"].(bool); !ok {
		t.Fatalf("passkeys: %s", raw)
	}
	code, pkOn, raw = do(http.MethodPut, "/api/v1/settings/passkeys", map[string]any{"enabled": true})
	requireStatus("passkeys put", code, http.StatusOK, pkOn, raw)
	if pkOn["enabled"] != true {
		t.Fatalf("passkeys put: %s", raw)
	}
	code, pkPublic, raw := call("", http.MethodGet, "/api/v1/auth/passkeys/enabled", nil)
	requireStatus("passkeys public", code, http.StatusOK, pkPublic, raw)
	if pkPublic["enabled"] != true {
		t.Fatalf("public passkeys: %s", raw)
	}

	code, policy, raw := do(http.MethodGet, "/api/v1/settings/image-scanning", nil)
	requireStatus("image scan policy", code, http.StatusOK, policy, raw)
	if field(policy, "gate") == "" && policy["enabled"] == nil {
		t.Fatalf("scan policy: %s", raw)
	}
	code, policy, raw = do(http.MethodPut, "/api/v1/settings/image-scanning", map[string]any{"enabled": true, "gate": "warn"})
	requireStatus("put image scan policy", code, http.StatusOK, policy, raw)
	if field(policy, "gate") != "warn" || policy["enabled"] != true {
		t.Fatalf("scan policy put: %s", raw)
	}
	code, ignored, raw := do(http.MethodPost, "/api/v1/image-scans/ignores", map[string]string{
		"vuln_id": "CVE-2024-0001", "reason": "accepted risk",
	})
	requireStatus("ignore vuln", code, http.StatusOK, ignored, raw)
	if field(ignored, "status") != "ignored" {
		t.Fatalf("ignore: %s", raw)
	}
	code, _, raw = do(http.MethodDelete, "/api/v1/image-scans/ignores/CVE-2024-0001", nil)
	if code != http.StatusNoContent {
		t.Fatalf("delete ignore: status %d body %s", code, raw)
	}

	appUUID, err := uuid.Parse(appID)
	if err != nil {
		t.Fatal(err)
	}
	dep, err := st.CreateDeployment(ctx, teamID, appUUID, nil, "abc123", "ship pulse-web", false, false, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	code, deps, raw := do(http.MethodGet, "/api/v1/applications/"+appID+"/deployments", nil)
	requireStatus("list deployments", code, http.StatusOK, deps, raw)
	if !strings.Contains(raw, dep.ID.String()) || !strings.Contains(raw, "queued") {
		t.Fatalf("deployments: %s", raw)
	}
	code, gotDep, raw := do(http.MethodGet, "/api/v1/deployments/"+dep.ID.String(), nil)
	requireStatus("get deployment", code, http.StatusOK, gotDep, raw)
	if field(gotDep, "status") != "queued" || field(gotDep, "commit_sha") != "abc123" {
		t.Fatalf("deployment: %s", raw)
	}
	code, cancelled, raw := do(http.MethodPost, "/api/v1/deployments/"+dep.ID.String()+"/cancel", nil)
	requireStatus("cancel deployment", code, http.StatusOK, cancelled, raw)
	if field(cancelled, "status") != "cancelled" {
		t.Fatalf("cancel: %s", raw)
	}
	code, gotDep, raw = do(http.MethodGet, "/api/v1/deployments/"+dep.ID.String(), nil)
	requireStatus("get cancelled deployment", code, http.StatusOK, gotDep, raw)
	if field(gotDep, "status") != "cancelled" {
		t.Fatalf("cancelled deployment: %s", raw)
	}

	code, overview, raw := do(http.MethodGet, "/api/v1/overview", nil)
	requireStatus("overview", code, http.StatusOK, overview, raw)
	if !strings.Contains(raw, "pulse-web") {
		t.Fatalf("overview missing app name: %s", raw)
	}
	counts, _ := overview["counts"].(map[string]any)
	if counts == nil || counts["applications"] == nil {
		t.Fatalf("overview counts: %s", raw)
	}

	code, audit, raw := do(http.MethodGet, "/api/v1/audit-logs?limit=20", nil)
	requireStatus("audit logs", code, http.StatusOK, audit, raw)
	logs, _ := audit["audit_logs"].([]any)
	total, _ := audit["total"].(float64)
	if len(logs) == 0 || total < 1 {
		t.Fatalf("audit logs: %s", raw)
	}
	first, _ := logs[0].(map[string]any)
	if first == nil || first["action"] == nil || first["category"] == nil {
		t.Fatalf("audit row: %s", raw)
	}

	hiddenProject, hiddenEnv, err := st.CreateProject(ctx, teamID, "hidden-project", "secret-hidden-name")
	if err != nil {
		t.Fatal(err)
	}
	hiddenApp := mustApp(t, st, teamID, hiddenEnv.ID, "secret-hidden-app")
	member, _, err := st.CreateUserWithPersonalTeam(ctx, "viewer@example.com", "Viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, access_scope) VALUES ($1,$2,'member','projects')`, teamID, member.ID); err != nil {
		t.Fatal(err)
	}
	visibleID, err := uuid.Parse(projectID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProjectAccess(ctx, teamID, member.ID, "projects", []store.ProjectGrant{{ProjectID: visibleID, Permission: "view"}}); err != nil {
		t.Fatal(err)
	}
	viewToken := mintSession(t, st, member.ID, teamID)
	if _, err := st.GetSession(ctx, viewToken); err != nil {
		t.Fatal(err)
	}

	code, viewProjects, raw := call(viewToken, http.MethodGet, "/api/v1/projects", nil)
	requireStatus("viewer projects", code, http.StatusOK, viewProjects, raw)
	if strings.Contains(raw, "hidden-project") || strings.Contains(raw, hiddenProject.ID.String()) {
		t.Fatalf("viewer saw hidden project: %s", raw)
	}
	if !strings.Contains(raw, "visible-renamed") {
		t.Fatalf("viewer missing granted project: %s", raw)
	}
	code, viewOverview, raw := call(viewToken, http.MethodGet, "/api/v1/overview", nil)
	requireStatus("viewer overview", code, http.StatusOK, viewOverview, raw)
	if strings.Contains(raw, "hidden-project") || strings.Contains(raw, "secret-hidden-app") {
		t.Fatalf("viewer overview leaked hidden project: %s", raw)
	}
	code, denied, raw := call(viewToken, http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": appID, "key": "LEAK", "value": "1", "is_runtime": true,
	})
	if code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("view-only write on granted app: status %d body %s", code, raw)
	}
	if strings.TrimSpace(field(denied, "error")) == "" {
		t.Fatalf("view-only write missing error: %s", raw)
	}
	code, deniedHidden, raw := call(viewToken, http.MethodPost, "/api/v1/env-vars", map[string]any{
		"resource_type": "application", "resource_id": hiddenApp.ID.String(), "key": "LEAK", "value": "1", "is_runtime": true,
	})
	if code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("view-only write on hidden app: status %d body %s", code, raw)
	}
	if strings.TrimSpace(field(deniedHidden, "error")) == "" {
		t.Fatalf("hidden write missing error: %s", raw)
	}
	code, freezeDenied, raw := call(viewToken, http.MethodPut, "/api/v1/projects/"+projectID+"/environments/"+envID+"/freeze", map[string]any{
		"frozen": true, "reason": "nope",
	})
	if code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("viewer freeze: status %d body %s", code, raw)
	}
	if strings.TrimSpace(field(freezeDenied, "error")) == "" {
		t.Fatalf("viewer freeze error: %s", raw)
	}
	_ = owner
}

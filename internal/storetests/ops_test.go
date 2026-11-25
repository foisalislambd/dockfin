package storetests

import (
	"errors"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestNotificationsDNSCloudAndOAuth(t *testing.T) {
	w := newWorld(t, "ops@example.com", "ops")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID
	app := w.app(t, "web", "dockerfile")

	setting, err := st.UpsertNotificationSetting(ctx, teamID, "slack", true, "enc-slack", []string{"deployment_failed"})
	if err != nil || !setting.Enabled || setting.Channel != "slack" || setting.ConfigEnc != "enc-slack" || len(setting.Events) != 1 || setting.Events[0] != "deployment_failed" {
		t.Fatalf("setting %+v %v", setting, err)
	}
	got, err := st.GetNotificationSetting(ctx, teamID, "slack")
	if err != nil || got.ID != setting.ID || !got.Enabled {
		t.Fatalf("get setting %+v %v", got, err)
	}
	enabled, err := st.ListEnabledNotifications(ctx, teamID)
	if err != nil || len(enabled) != 1 || enabled[0].Channel != "slack" {
		t.Fatalf("enabled %+v %v", enabled, err)
	}
	if _, err := st.UpsertNotificationSetting(ctx, teamID, "slack", false, "enc-slack", []string{"deployment_success", "deployment_failed"}); err != nil {
		t.Fatal(err)
	}
	enabled, err = st.ListEnabledNotifications(ctx, teamID)
	if err != nil || len(enabled) != 0 {
		t.Fatalf("still enabled %+v %v", enabled, err)
	}
	all, err := st.ListNotificationSettings(ctx, teamID)
	if err != nil || len(all) != 1 || all[0].Enabled || len(all[0].Events) != 2 {
		t.Fatalf("all settings %+v %v", all, err)
	}
	if _, err := st.GetNotificationSetting(ctx, teamID, "email"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing channel %v", err)
	}

	user := "mailer"
	pass := "mailer-pass"
	resend := "re_test_key"
	if _, err := st.UpdateInstanceSettings(ctx, store.InstanceSettingsPatch{SMTPUsername: &user, SMTPPassword: &pass, ResendAPIKey: &resend}); err != nil {
		t.Fatal(err)
	}
	gotUser, gotPass, gotKey, err := st.SMTPMaterial(ctx)
	if err != nil || gotUser != user || gotPass != pass || gotKey != resend {
		t.Fatalf("smtp %q %q %q %v", gotUser, gotPass, gotKey, err)
	}

	token, err := st.CreateCloudProviderToken(ctx, teamID, "hetzner", "prod", "primary", encrypt(t, st, "hz-token"))
	if err != nil || token.Provider != "hetzner" || token.Name != "prod" {
		t.Fatalf("token %+v %v", token, err)
	}
	if _, err := st.CreateCloudProviderToken(ctx, teamID, "hetzner", "prod", "", "x"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate token %v", err)
	}
	material, err := st.GetCloudProviderTokenMaterial(ctx, teamID, token.ID)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := st.Box.DecryptString(material)
	if err != nil || plain != "hz-token" {
		t.Fatalf("token material %q %v", plain, err)
	}
	renamedToken, err := st.UpdateCloudProviderToken(ctx, teamID, token.ID, "prod-renamed", "secondary", nil)
	if err != nil || renamedToken.Name != "prod-renamed" || renamedToken.Description != "secondary" {
		t.Fatalf("renamed token %+v %v", renamedToken, err)
	}
	tokens, err := st.ListCloudProviderTokens(ctx, teamID)
	if err != nil || len(tokens) != 1 || tokens[0].Name != "prod-renamed" {
		t.Fatalf("tokens %+v %v", tokens, err)
	}

	zones, retained, err := st.ReplaceDNSZones(ctx, teamID, token.ID, []store.DNSZoneInput{
		{ProviderZoneID: "z1", Name: "Example.com"},
		{ProviderZoneID: "z2", Name: "other.com"},
	})
	if err != nil || len(zones) != 2 || len(retained) != 0 {
		t.Fatalf("zones %+v retained %v %v", zones, retained, err)
	}
	listedZones, err := st.ListDNSZones(ctx, teamID)
	if err != nil || len(listedZones) != 2 || listedZones[0].Name != "example.com" {
		t.Fatalf("listed zones %+v %v", listedZones, err)
	}
	example := mustFind(t, zones, func(z store.DNSZone) bool { return z.Name == "example.com" })
	record, err := st.SaveDNSRecord(ctx, store.DNSRecord{
		ID: uuid.New(), TeamID: teamID, ZoneID: example.ID, TokenID: token.ID,
		ProviderRecordID: "rec-1", Type: "A", Name: "app.example.com", Content: "203.0.113.9", Owned: true, Comment: "web",
	})
	if err != nil || record.ProviderZoneID != "z1" || record.Content != "203.0.113.9" || record.Name != "app.example.com" {
		t.Fatalf("record %+v %v", record, err)
	}
	if err := st.AddDNSRef(ctx, record.ID, "application", app.ID); err != nil {
		t.Fatal(err)
	}
	svc := w.service(t, "n8n")
	if err := st.AddDNSRef(ctx, record.ID, "service", svc.ID); err != nil {
		t.Fatal(err)
	}
	others, err := st.OtherDNSRefs(ctx, record.ID, "application", app.ID)
	if err != nil || others != 1 {
		t.Fatalf("other refs %d %v", others, err)
	}
	hosts, err := st.DNSHostsForResource(ctx, teamID, "application", app.ID)
	if err != nil || len(hosts) != 1 || hosts[0] != "app.example.com" {
		t.Fatalf("hosts %#v %v", hosts, err)
	}
	left, err := st.RemoveDNSRef(ctx, record.ID, "application", app.ID)
	if err != nil || left != 1 {
		t.Fatalf("refs left %d %v", left, err)
	}
	byName, err := st.DNSRecordByName(ctx, teamID, "app.example.com")
	if err != nil || byName == nil || byName.ID != record.ID || byName.Type != "A" {
		t.Fatalf("by name %+v %v", byName, err)
	}
	if missing, err := st.DNSRecordByName(ctx, teamID, "missing.example.com"); err != nil || missing != nil {
		t.Fatalf("missing record %+v %v", missing, err)
	}
	zones, retained, err = st.ReplaceDNSZones(ctx, teamID, token.ID, []store.DNSZoneInput{{ProviderZoneID: "z2", Name: "other.com"}})
	if err != nil || len(retained) != 1 || retained[0] != "example.com" {
		t.Fatalf("retained %+v zones %+v %v", retained, zones, err)
	}
	if len(zones) != 2 {
		t.Fatalf("example.com should stay while it has records: %+v", zones)
	}
	if err := st.DeleteDNSRecord(ctx, record.ID); err != nil {
		t.Fatal(err)
	}
	hosts, err = st.DNSHostsForResource(ctx, teamID, "service", svc.ID)
	if err != nil || len(hosts) != 0 {
		t.Fatalf("hosts after delete %#v %v", hosts, err)
	}
	zones, retained, err = st.ReplaceDNSZones(ctx, teamID, token.ID, nil)
	if err != nil || len(zones) != 0 || len(retained) != 0 {
		t.Fatalf("cleared zones %+v retained %v %v", zones, retained, err)
	}

	script, err := st.CreateCloudInitScript(ctx, teamID, "boot", encrypt(t, st, "#!/bin/sh\necho hi\n"))
	if err != nil || script.Name != "boot" {
		t.Fatalf("script %+v %v", script, err)
	}
	updated, err := st.UpdateCloudInitScript(ctx, teamID, script.ID, "boot2", encrypt(t, st, "#!/bin/sh\necho bye\n"))
	if err != nil || updated.Name != "boot2" {
		t.Fatalf("updated script %+v %v", updated, err)
	}
	loaded, enc, err := st.GetCloudInitScript(ctx, teamID, script.ID)
	if err != nil || loaded.Name != "boot2" {
		t.Fatalf("loaded script %+v %v", loaded, err)
	}
	body, err := st.Box.DecryptString(enc)
	if err != nil || !strings.Contains(body, "echo bye") {
		t.Fatalf("script body %q %v", body, err)
	}
	scripts, err := st.ListCloudInitScripts(ctx, teamID)
	if err != nil || len(scripts) != 1 || scripts[0].ID != script.ID {
		t.Fatalf("scripts %+v %v", scripts, err)
	}
	if err := st.DeleteCloudInitScript(ctx, teamID, script.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetCloudInitScript(ctx, teamID, script.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("script remains %v", err)
	}
	if err := st.DeleteCloudProviderToken(ctx, teamID, token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetCloudProviderToken(ctx, teamID, token.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("token remains %v", err)
	}

	secret := "oauth-secret"
	on := true
	clientID := "github-client"
	redirect := "https://panel.example.com/callback"
	patched, err := st.UpdateOauthSetting(ctx, "github", store.OauthSettingPatch{
		Enabled: &on, ClientID: &clientID, ClientSecret: &secret, RedirectURI: &redirect,
	})
	if err != nil || !patched.Enabled || patched.ClientID != clientID || !patched.ClientSecretSet {
		t.Fatalf("oauth %+v %v", patched, err)
	}
	materialOAuth, err := st.GetOauthSettingMaterial(ctx, "github")
	if err != nil || materialOAuth.ClientSecret != secret || materialOAuth.RedirectURI != redirect || !materialOAuth.Enabled {
		t.Fatalf("oauth material %+v %v", materialOAuth, err)
	}
	providers, err := st.ListEnabledOauthProviders(ctx)
	if err != nil || len(providers) != 1 || providers[0].Provider != "github" || providers[0].Name != "GitHub" {
		t.Fatalf("providers %+v %v", providers, err)
	}
	if err := st.LinkOauthAccount(ctx, w.user.ID, "github", "gh-42", "Ops@Example.com"); err != nil {
		t.Fatal(err)
	}
	linked, err := st.FindOauthAccountUser(ctx, "github", "gh-42")
	if err != nil || linked.ID != w.user.ID || linked.Email != "ops@example.com" {
		t.Fatalf("linked %+v %v", linked, err)
	}
	if _, err := st.FindOauthAccountUser(ctx, "github", "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing oauth user %v", err)
	}
	settings, err := st.ListOauthSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	github := mustFind(t, settings, func(o store.OauthSetting) bool { return o.Provider == "github" })
	if !github.Enabled || github.ClientID != clientID {
		t.Fatalf("github setting %+v", github)
	}
}

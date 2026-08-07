package store

import (
	"errors"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestApplyInstanceSettingsPatch_General(t *testing.T) {
	cur := &InstanceSettings{
		PublicURL:        "",
		InstanceName:     "Dockfin",
		InstanceTimezone: "UTC",
		SMTPFromAddress:  "bad-not-an-email", // must not fail general patch
		SMTPEnabled:      true,
	}
	err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{
		PublicURL:        ptr("https://dash.example.com/path?x=1"),
		InstanceName:     ptr(" My Cloud "),
		InstanceTimezone: ptr("Asia/Dhaka"),
		PublicIPv4:       ptr("1.2.3.4"),
		PublicIPv6:       ptr("2001:db8::1"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cur.PublicURL != "https://dash.example.com" {
		t.Fatalf("public url strip path: got %q", cur.PublicURL)
	}
	if cur.InstanceName != "My Cloud" {
		t.Fatalf("name: got %q", cur.InstanceName)
	}
	if cur.InstanceTimezone != "Asia/Dhaka" {
		t.Fatalf("tz: got %q", cur.InstanceTimezone)
	}
	if cur.PublicIPv4 != "1.2.3.4" || cur.PublicIPv6 != "2001:db8::1" {
		t.Fatalf("ips: %q %q", cur.PublicIPv4, cur.PublicIPv6)
	}
	// Existing invalid SMTP address must not block general save.
	if cur.SMTPFromAddress != "bad-not-an-email" {
		t.Fatal("smtp address should be untouched")
	}

	// Bare hostname → https://…
	cur2 := &InstanceSettings{InstanceName: "Dockfin", InstanceTimezone: "UTC"}
	if err := applyInstanceSettingsPatch(cur2, &InstanceSettingsPatch{PublicURL: ptr("dash.example.com")}); err != nil {
		t.Fatalf("bare domain: %v", err)
	}
	if cur2.PublicURL != "https://dash.example.com" {
		t.Fatalf("bare domain normalize: got %q", cur2.PublicURL)
	}
}

func TestApplyInstanceSettingsPatch_RejectsBadURLAndIPs(t *testing.T) {
	cur := &InstanceSettings{InstanceName: "Dockfin", InstanceTimezone: "UTC"}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{PublicURL: ptr("://")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for bad url, got %v", err)
	}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{PublicURL: ptr("http://")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for empty host, got %v", err)
	}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{PublicIPv4: ptr("2001:db8::1")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for ipv6 in ipv4 field, got %v", err)
	}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{PublicIPv6: ptr("1.2.3.4")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for ipv4 in ipv6 field, got %v", err)
	}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{InstanceTimezone: ptr("Not/AZone")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for bad tz, got %v", err)
	}
}

func TestApplyInstanceSettingsPatch_DNSAndRegistry(t *testing.T) {
	cur := &InstanceSettings{CustomDNSServers: "1.1.1.1", DockerRegistryURL: "ghcr.io", UpdateChannel: "stable"}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{CustomDNSServers: ptr("1.1.1.1, not-an-ip")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for bad dns, got %v", err)
	}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{
		CustomDNSServers:  ptr("1.1.1.1, 8.8.8.8"),
		DockerRegistryURL: ptr("docker.io"),
		UpdateChannel:     ptr("next"),
	}); err != nil {
		t.Fatal(err)
	}
	if cur.CustomDNSServers != "1.1.1.1, 8.8.8.8" || cur.DockerRegistryURL != "docker.io" || cur.UpdateChannel != "next" {
		t.Fatalf("unexpected: %+v", cur)
	}
}

func TestApplyInstanceSettingsPatch_SMTPEmailAndMutex(t *testing.T) {
	cur := &InstanceSettings{SMTPEnabled: false, ResendEnabled: false}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{SMTPFromAddress: ptr("nope")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict for bad email, got %v", err)
	}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{
		SMTPEnabled:     ptr(true),
		ResendEnabled:   ptr(true),
		SMTPFromAddress: ptr("ops@example.com"),
		SMTPPort:        ptr(465),
		SMTPEncryption:  ptr("tls"),
	}); err != nil {
		t.Fatal(err)
	}
	if !cur.ResendEnabled || cur.SMTPEnabled {
		t.Fatalf("resend enable should win mutex: smtp=%v resend=%v", cur.SMTPEnabled, cur.ResendEnabled)
	}
	if cur.SMTPFromAddress != "ops@example.com" || cur.SMTPPort != 465 || cur.SMTPEncryption != "tls" {
		t.Fatalf("smtp fields: %+v", cur)
	}

	// Enabling SMTP while Resend is on should disable Resend.
	cur.SMTPEnabled = false
	cur.ResendEnabled = true
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{SMTPEnabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if !cur.SMTPEnabled || cur.ResendEnabled {
		t.Fatalf("smtp enable should clear resend: smtp=%v resend=%v", cur.SMTPEnabled, cur.ResendEnabled)
	}
}

func TestApplyInstanceSettingsPatch_EmptyURLClears(t *testing.T) {
	cur := &InstanceSettings{PublicURL: "https://old.example.com"}
	if err := applyInstanceSettingsPatch(cur, &InstanceSettingsPatch{PublicURL: ptr("  ")}); err != nil {
		t.Fatal(err)
	}
	if cur.PublicURL != "" {
		t.Fatalf("expected cleared url, got %q", cur.PublicURL)
	}
}

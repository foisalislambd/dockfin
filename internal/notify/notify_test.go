package notify

import "testing"

func TestEventAllowedAliases(t *testing.T) {
	events := []string{"deployment_failure", "backup_success"}
	if !EventAllowed(events, "deployment_failed") {
		t.Fatal("deployment_failed should match deployment_failure")
	}
	if !EventAllowed(events, "deployment_failure") {
		t.Fatal("exact match should work")
	}
	if EventAllowed(events, "backup_failure") {
		t.Fatal("backup_failure should not match backup_success only list")
	}
	if EventAllowed(nil, "anything") {
		t.Fatal("nil events should allow nothing")
	}
	if EventAllowed([]string{}, "anything") {
		t.Fatal("empty selection should allow nothing")
	}
}

func TestNormalizeEvent(t *testing.T) {
	if NormalizeEvent("deployment_failed") != "deployment_failure" {
		t.Fatal("normalize deployment_failed")
	}
	if NormalizeEvent("DEPLOYMENT_FAILURE") != "deployment_failure" {
		t.Fatal("normalize case")
	}
	got := NormalizeEvents([]string{"deployment_failed", "backup_failure", "deployment_failure"})
	if len(got) != 2 {
		t.Fatalf("dedupe expected 2 got %v", got)
	}
}

func TestIsCritical(t *testing.T) {
	if !IsCritical("deployment_failure") || IsCritical("deployment_success") {
		t.Fatal("critical classification wrong")
	}
}

func TestWebhookSuccessFlag(t *testing.T) {
	// success should be true for non-critical and for test
	if IsCritical("deployment_success") {
		t.Fatal("success not critical")
	}
	if !IsCritical("test") {
		t.Fatal("test marked critical for ping, but webhook payload special-cases it")
	}
}

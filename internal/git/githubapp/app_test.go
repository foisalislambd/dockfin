package githubapp

import "testing"

func TestAPIURLFromHTML(t *testing.T) {
	if got := APIURLFromHTML("https://github.com"); got != "https://api.github.com" {
		t.Fatalf("github.com => %s", got)
	}
	if got := APIURLFromHTML("https://git.company.internal"); got != "https://git.company.internal/api/v3" {
		t.Fatalf("self-hosted => %s", got)
	}
	if got := APIURLFromHTML(""); got != "https://api.github.com" {
		t.Fatalf("empty => %s", got)
	}
}

func TestToSSHURL(t *testing.T) {
	got := ToSSHURL("https://github.com/acme/app.git", "git")
	if got != "git@github.com:acme/app.git" {
		t.Fatalf("got %s", got)
	}
	if ToSSHURL("git@github.com:acme/app.git", "git") != "git@github.com:acme/app.git" {
		t.Fatal("ssh passthrough")
	}
}

func TestCloneURL(t *testing.T) {
	got := CloneURL("https://github.com/acme/app.git", "tok")
	if got != "https://x-access-token:tok@github.com/acme/app.git" {
		t.Fatalf("got %s", got)
	}
}

func TestBuildManifestSetupURL(t *testing.T) {
	m := BuildManifest("my-app", "https://dash.example.com", "/github/app/events", "/github/app/manifest", "/github/app/callback", "src-uuid", true)
	setup, _ := m["setup_url"].(string)
	if setup != "https://dash.example.com/api/v1/webhooks/github/app/callback?source_id=src-uuid" {
		t.Fatalf("setup_url=%s", setup)
	}
	hook, _ := m["hook_attributes"].(map[string]any)
	if hook["url"] != "https://dash.example.com/api/v1/webhooks/github/app/events" {
		t.Fatalf("hook=%v", hook)
	}
	perms, _ := m["default_permissions"].(map[string]string)
	if perms["pull_requests"] != "write" {
		t.Fatal("preview permissions missing")
	}
}

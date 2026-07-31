package deploy

import "testing"

func TestNormalizeHTTPSRepo(t *testing.T) {
	if got := normalizeHTTPSRepo("acme/app", "https://github.com"); got != "https://github.com/acme/app.git" {
		t.Fatalf("got %s", got)
	}
	if got := normalizeHTTPSRepo("https://github.com/acme/app.git", ""); got != "https://github.com/acme/app.git" {
		t.Fatalf("passthrough https got %s", got)
	}
	if got := normalizeHTTPSRepo("git@github.com:acme/app.git", ""); got != "git@github.com:acme/app.git" {
		t.Fatalf("passthrough ssh got %s", got)
	}
}

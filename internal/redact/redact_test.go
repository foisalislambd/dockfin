package redact

import (
	"strings"
	"testing"
)

func TestSecretsRedactsGitHubCloneURL(t *testing.T) {
	in := `fatal: repository 'https://x-access-token:ghs_ABC123secrettoken@github.com/acme/app.git' not found`
	out := Secrets(in)
	if strings.Contains(out, "ghs_ABC123") || strings.Contains(out, "secrettoken") {
		t.Fatalf("token leaked: %q", out)
	}
	if !strings.Contains(out, "x-access-token:***@") {
		t.Fatalf("expected redaction marker, got %q", out)
	}
}

func TestSecretsRedactsUserPass(t *testing.T) {
	in := `clone failed https://user:pAssw0rd@gitlab.com/a/b.git`
	out := Secrets(in)
	if strings.Contains(out, "pAssw0rd") {
		t.Fatalf("password leaked: %q", out)
	}
}

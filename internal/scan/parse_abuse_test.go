package scan

import "testing"

func TestParseRepoRejectsAbuse(t *testing.T) {
	good, err := ParseRepo("https://github.com/acme/web.git")
	if err != nil || good.Owner != "acme" || good.Name != "web" || good.Host != "github.com" {
		t.Fatal(good, err)
	}
	short, err := ParseRepo("acme/web")
	if err != nil || short.Host != "github.com" || short.Name != "web" {
		t.Fatal(short, err)
	}
	ssh, err := ParseRepo("git@gitlab.com:acme/web.git")
	if err != nil || ssh.Host != "gitlab.com" || ssh.Owner != "acme" {
		t.Fatal(ssh, err)
	}
	for _, raw := range []string{"", "acme", "https://github.com/acme", "https://169.254.169.254/latest/meta", "/etc/passwd", "acme/../web"} {
		if _, err := ParseRepo(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

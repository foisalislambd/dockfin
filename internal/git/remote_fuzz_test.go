package git

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzValidateRemote(f *testing.F) {
	f.Add("https://github.com/acme/web.git")
	f.Add("git@github.com:acme/web.git")
	f.Add("file:///etc/passwd")
	f.Add("-c protocol.ext.allow=always")
	f.Add("https://169.254.169.254/latest")
	f.Add("ext::sh -c id")
	f.Fuzz(func(t *testing.T, raw string) {
		if err := ValidateRemote(raw); err != nil {
			return
		}
		trimmed := strings.TrimSpace(raw)
		if strings.ContainsAny(trimmed, " \t\r\n'\\") || strings.HasPrefix(trimmed, "-") || strings.Contains(trimmed, "..") {
			t.Fatalf("accepted %q", raw)
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "ext:") {
			t.Fatalf("accepted %q", raw)
		}
		host := lower
		if u, err := url.Parse(trimmed); err == nil && u.Hostname() != "" {
			host = strings.ToLower(u.Hostname())
		}
		if strings.Contains(host, "169.254.169.254") || BlockedFetchHost(host) {
			t.Fatalf("accepted %q", raw)
		}
	})
}

func TestGitRemoteRejectsSSRFAndInjection(t *testing.T) {
	bad := []string{
		"",
		"file:///etc/passwd",
		"/etc/passwd",
		"-c core.sshCommand=id",
		"ext::sh -c id",
		"https://169.254.169.254/latest/meta-data",
		"http://0169.254.169.254",
		"http://169.254.169.2540",
		"http://metadata.google.internal/computeMetadata/v1/",
		"https://github.com/acme/../secret.git",
		"https://user:token@github.com/acme/web.git\nid",
		"git@github.com:acme/web.git;id",
		"file:/0",
		"file:/etc/passwd",
		"169.254.169.254/0",
		"0169.254.169.254/0",
		"ssh://git@github.com/acme/web.git\tid",
	}
	for _, raw := range bad {
		if err := ValidateRemote(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, raw := range []string{
		"https://github.com/acme/web.git",
		"http://github.com/acme/web.git",
		"ssh://git@github.com/acme/web.git",
		"git@github.com:acme/web.git",
		"acme/web",
	} {
		if err := ValidateRemote(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}

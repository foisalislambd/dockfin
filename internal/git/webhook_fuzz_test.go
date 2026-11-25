package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
)

func FuzzVerifyGitHubSignature(f *testing.F) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	f.Add("secret", body, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	f.Add("secret", body, "sha256=00")
	f.Add("", body, "")
	f.Fuzz(func(t *testing.T, secret string, body []byte, header string) {
		ok := VerifyGitHubSignature(secret, body, header)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		got := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(header)), "sha256=")
		if ok && !hmac.Equal([]byte(want), []byte(got)) {
			t.Fatalf("accepted %q for %q", header, want)
		}
		if hmac.Equal([]byte(want), []byte(got)) && header != "" && !ok && len(want) == len(got) {
			t.Fatalf("rejected a matching signature %q", header)
		}
	})
}

func FuzzParseWebhook(f *testing.F) {
	f.Add("github", "push", []byte(`{"ref":"refs/heads/main","after":"abc","repository":{"full_name":"acme/web"}}`))
	f.Add("gitlab", "Push Hook", []byte(`{"object_kind":"push","ref":"refs/heads/main","project":{"path_with_namespace":"acme/web"}}`))
	f.Add("gitlab", "System Hook", []byte(`{"event_name":"push","project_id":1}`))
	f.Add("github", "pull_request", []byte(`{"action":"opened","number":1,"pull_request":{"head":{"ref":"f","sha":"abc"},"base":{"ref":"main"}}}`))
	f.Fuzz(func(t *testing.T, provider, event string, body []byte) {
		req, err := http.NewRequest(http.MethodPost, "http://example.test/hook", nil)
		if err != nil {
			t.Fatal(err)
		}
		switch provider {
		case "github":
			req.Header.Set("X-GitHub-Event", event)
		case "gitlab":
			req.Header.Set("X-Gitlab-Event", event)
		}
		if _, err := ParseWebhook(provider, req, body); err != nil {
			return
		}
		norm, err := NormalizeGitLabSystemHook(body)
		if provider == "gitlab" && err == nil && strings.Contains(string(norm), "\x00") {
			t.Fatal("normalized hook kept a NUL")
		}
	})
}

func TestWebhookSignaturesRejectTamper(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	good := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifyGitHubSignature("secret", body, good) {
		t.Fatal("valid signature rejected")
	}
	last := good[len(good)-1]
	var repl byte
	switch {
	case last == '9':
		repl = 'a'
	case last == 'f' || last == 'F':
		repl = '0'
	default:
		repl = last + 1
	}
	flipped := good[:len(good)-1] + string(repl)
	for _, bad := range []string{"", "sha256=", flipped, "sha1=" + good[7:]} {
		if VerifyGitHubSignature("secret", body, bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
	if VerifyGitHubSignature("other", body, good) {
		t.Fatal("accepted a signature for the wrong secret")
	}
	if !VerifyGitHubSignature("secret", body, strings.ToUpper(good)) {
		t.Fatal("hex compare should be case-insensitive")
	}
}

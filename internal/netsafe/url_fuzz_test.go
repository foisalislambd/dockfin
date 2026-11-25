package netsafe

import (
	"net"
	"net/url"
	"strings"
	"testing"
)

func FuzzValidateHTTPURL(f *testing.F) {
	f.Add("https://example.com/hook", false)
	f.Add("http://169.254.169.254/latest/meta-data", true)
	f.Add("http://127.0.0.1:9000/bucket", true)
	f.Add("http://user:pass@example.com", false)
	f.Add("file:///etc/passwd", false)
	f.Add("https://metadata.google.internal/computeMetadata/v1/", false)
	f.Fuzz(func(t *testing.T, raw string, allowPrivate bool) {
		norm, err := ValidateHTTPURL(raw, allowPrivate)
		if err != nil {
			return
		}
		u, err := url.Parse(norm)
		if err != nil || u.User != nil {
			t.Fatalf("normalized %q", norm)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			t.Fatalf("scheme %s", u.Scheme)
		}
		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		if blockedMetadataHost(host) {
			t.Fatalf("metadata host %s", host)
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
				t.Fatalf("blocked ip %s", ip)
			}
			if !allowPrivate && (ip.IsLoopback() || ip.IsPrivate()) {
				t.Fatalf("private ip %s", ip)
			}
		}
	})
}

func TestURLAndSSRFBlocked(t *testing.T) {
	blocked := []string{
		"http://169.254.169.254/latest/meta-data",
		"http://0169.254.169.254/",
		"http://169.254.169.2540/",
		"http://[::ffff:169.254.169.254]/",
		"https://metadata.google.internal/",
		"https://metadata.google.com/",
		"http://127.0.0.1/",
		"http://10.0.0.5/",
		"http://192.168.1.1/",
		"http://[::1]/",
		"http://0.0.0.0/",
		"file:///etc/passwd",
		"gopher://example.com",
		"http://user:pass@example.com/hook",
		"https://example.com\\@169.254.169.254/",
		"",
		"http://",
	}
	for _, raw := range blocked {
		if _, err := ValidateHTTPURL(raw, false); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := ValidateHTTPURL("http://127.0.0.1:9000/minio", true); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateHTTPURL("http://169.254.169.254/", true); err == nil {
		t.Fatal("metadata allowed when private is allowed")
	}
	got, err := ValidateHTTPURL("https://Example.COM/path/", false)
	if err != nil || got != "https://example.com/path" {
		t.Fatalf("normalized %q %v", got, err)
	}
}

package proxy

import (
	"strings"
	"testing"
)

func FuzzNormalizeDomains(f *testing.F) {
	f.Add("https://Example.COM/path")
	f.Add("a.com,b.com")
	f.Add("http://127.0.0.1")
	f.Add("not a host")
	f.Add("javascript:alert(1)")
	f.Add("a.com\nb.com")
	f.Fuzz(func(t *testing.T, s string) {
		out := NormalizeDomains(s)
		lower := strings.ToLower(out)
		for _, bad := range []string{"javascript:", " ", "\n", "\r", ".."} {
			if strings.Contains(lower, bad) {
				t.Fatalf("NormalizeDomains(%q) = %q contains %q", s, out, bad)
			}
		}
	})
}

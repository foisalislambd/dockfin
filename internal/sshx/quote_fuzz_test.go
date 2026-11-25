package sshx

import (
	"strings"
	"testing"
	"unicode"
)

func TestShellQuoteRoundTrip(t *testing.T) {
	samples := []string{
		"",
		"nginx",
		"a b",
		"it's",
		"$(rm -rf /)",
		"`id`",
		"a;b",
		"line\nbreak",
		"tab\there",
		`a'"'"'b`,
		"../etc/passwd",
		"http://example.com",
		"name_1.2:3",
	}
	for _, s := range samples {
		got, ok := unquoteShellWord(Quote(s))
		if !ok || got != s {
			t.Fatalf("quote %q -> %q decoded %q ok=%v", s, Quote(s), got, ok)
		}
	}
}

func FuzzShellQuote(f *testing.F) {
	f.Add("")
	f.Add("ok")
	f.Add("a b;rm")
	f.Add("it's")
	f.Add("$(id)")
	f.Fuzz(func(t *testing.T, s string) {
		if strings.ContainsRune(s, 0) {
			t.Skip()
		}
		q := Quote(s)
		got, ok := unquoteShellWord(q)
		if !ok || got != s {
			t.Fatalf("quote %q -> %q decoded %q ok=%v", s, q, got, ok)
		}
		if strings.ContainsAny(q, "\n") && !strings.HasPrefix(q, "'") {
			t.Fatalf("newline left unquoted: %q", q)
		}
	})
}

// unquoteShellWord parses one POSIX word produced by shellQuote.
func unquoteShellWord(q string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(q); {
		if q[i] == '\'' {
			j := strings.IndexByte(q[i+1:], '\'')
			if j < 0 {
				return "", false
			}
			b.WriteString(q[i+1 : i+1+j])
			i += j + 2
			continue
		}
		// shellQuote encodes an embedded single quote as '"'"'.
		if strings.HasPrefix(q[i:], `"'"`) {
			b.WriteByte('\'')
			i += 3
			continue
		}
		start := i
		for i < len(q) && q[i] != '\'' {
			r := rune(q[i])
			if unicode.IsSpace(r) || strings.ContainsRune("&|;<>$`\\\"()*?[]{}!~#", r) {
				return "", false
			}
			i++
		}
		b.WriteString(q[start:i])
	}
	return b.String(), true
}

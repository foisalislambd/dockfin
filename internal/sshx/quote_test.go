package sshx

import (
	"strings"
	"testing"
)

func TestQuoteStopsShellMetacharacters(t *testing.T) {
	for _, raw := range []string{"ok", "a b", "a'b", "$(id)", "a;id", "a\nid", "a|id", ""} {
		q := Quote(raw)
		if raw == "ok" {
			if q != "ok" {
				t.Fatal(q)
			}
			continue
		}
		if !strings.HasPrefix(q, "'") || !strings.HasSuffix(q, "'") {
			t.Fatalf("%q -> %s", raw, q)
		}
	}
	if Quote("a'b") != `'a'"'"'b'` {
		t.Fatal(Quote("a'b"))
	}
}

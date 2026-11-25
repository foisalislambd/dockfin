package services

import (
	"strings"
	"testing"
)

func FuzzParseEnvFile(f *testing.F) {
	f.Add("PORT=80\n")
	f.Add("MSG=\"hello\\nworld\"\n")
	f.Add("BAD\nKEY=1\nKEY=2\n")
	f.Add("A=\"\\\\n\"\n")
	f.Add("# comment\nFOO=bar\n")
	f.Fuzz(func(t *testing.T, raw string) {
		got, err := ParseEnvFile(raw)
		if err != nil {
			return
		}
		for k, v := range got {
			if !envFileKey(k) || strings.ContainsAny(k, "\r\n=#") {
				t.Fatalf("accepted key %q", k)
			}
			again, err := ParseEnvFile(FormatEnvFile(map[string]string{k: v}))
			if err != nil {
				t.Fatalf("round trip %q=%q: %v", k, v, err)
			}
			if again[k] != v {
				t.Fatalf("round trip %q: got %q want %q", k, again[k], v)
			}
		}
	})
}

func TestEnvFileRejectsInjection(t *testing.T) {
	for _, raw := range []string{
		"A=1\rB=2",
		"KEY\nEVIL=1=x",
		" KEY=1\n",
		"KEY =1\n",
		"1KEY=1\n",
	} {
		if _, err := ParseEnvFile(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	out := FormatEnvFile(map[string]string{"GOOD": "1", "BAD\nEVIL": "2", "ALSO=BAD": "3"})
	if strings.Contains(out, "EVIL") || strings.Contains(out, "ALSO") {
		t.Fatalf("format wrote a hostile key:\n%s", out)
	}
	parsed, err := ParseEnvFile(out)
	if err != nil || parsed["GOOD"] != "1" || len(parsed) != 1 {
		t.Fatalf("parsed %v %v", parsed, err)
	}
}

package proxy

import (
	"strings"
	"testing"
)

func TestLabelsDropShellAndUnknownRedirects(t *testing.T) {
	labels := TraefikBasicAuthLabels("Web App; $(id)", "alice:hash")
	if len(labels) != 2 || strings.Contains(labels[0], "$(") || strings.Contains(labels[0], ";") {
		t.Fatalf("%v", labels)
	}
	if TraefikBasicAuthLabels("app", "") != nil || TraefikWWWRedirectLabels("app", "javascript", true) != nil {
		t.Fatal("accepted an empty user list or an unknown redirect")
	}
	www := TraefikWWWRedirectLabels("app", "WWW", false)
	if len(www) == 0 || !strings.Contains(strings.Join(www, "\n"), "302") && !strings.Contains(strings.Join(www, "\n"), "temporary") && !strings.Contains(strings.Join(www, "\n"), "to-www") {
		t.Fatalf("%v", www)
	}
	custom := ParseCustomLabels("# comment\ntraefik.enable=true, not-a-label\n\ncom.example=value")
	if len(custom) != 2 || custom[0] != "traefik.enable=true" || custom[1] != "com.example=value" {
		t.Fatalf("%v", custom)
	}
	if got := sanitize("Name\nHost"); strings.ContainsAny(got, "\n ") {
		t.Fatal(got)
	}
}

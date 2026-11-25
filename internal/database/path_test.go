package database

import (
	"strings"
	"testing"
)

func TestHostDataDirRejectsTraversal(t *testing.T) {
	if got := HostDataDir("abc-123"); got != "/data/dockfin/databases/abc-123" {
		t.Fatal(got)
	}
	for _, id := range []string{"", ".", "..", "../etc", "abc/../../etc", `abc\..\windows`, "abc/def", "..abc"} {
		got := HostDataDir(id)
		if !strings.HasSuffix(got, "/invalid") || strings.Contains(got, "..") {
			t.Fatalf("%q -> %s", id, got)
		}
	}
}

package backup

import (
	"strings"
	"testing"
)

func TestDumpPathRejectsTraversal(t *testing.T) {
	if got := DumpPath("pg.sql"); !strings.HasSuffix(got, "/pg.sql") || strings.Contains(got, "..") {
		t.Fatal(got)
	}
	for _, name := range []string{"../etc/passwd", "..", "a/b", `a\b`, "name\nfile", ""} {
		got := DumpPath(name)
		if !strings.HasSuffix(got, "/invalid") {
			t.Fatalf("%q -> %s", name, got)
		}
	}
	if !SafeFilename("backup-1.sql") || SafeFilename("..") || SafeFilename("a/b") {
		t.Fatal("SafeFilename")
	}
}

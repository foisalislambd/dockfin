package backup

import (
	"strings"
	"testing"
)

func TestParseDatabaseURL(t *testing.T) {
	u, p, d, err := ParseDatabaseURL("postgres://goolify:secret@127.0.0.1:5432/goolify?sslmode=disable")
	if err != nil || u != "goolify" || p != "secret" || d != "goolify" {
		t.Fatalf("got %q %q %q %v", u, p, d, err)
	}
}

func TestInstanceDumpFilenameUnique(t *testing.T) {
	a := InstanceDumpFilename()
	b := InstanceDumpFilename()
	if a == b {
		t.Fatalf("expected unique filenames, got %q", a)
	}
	if !strings.HasPrefix(a, "pg-dump-goolify-") || !strings.HasSuffix(a, ".sql") {
		t.Fatalf("unexpected format: %q", a)
	}
}

func TestDetectAndDump(t *testing.T) {
	c, err := DetectPostgresContainer("")
	if err != nil {
		t.Skip("no postgres container:", err)
	}
	dir := t.TempDir()
	fn := InstanceDumpFilename()
	path, size, err := DumpInstanceLocal(dir, c, "goolify", "goolify", "goolify", fn)
	if err != nil {
		t.Fatal(err)
	}
	if size <= 0 {
		t.Fatalf("expected size > 0, path=%s", path)
	}
	t.Log("ok", path, size)
}

func TestDetectPostgresContainerPrefersRunning(t *testing.T) {
	running, err := DetectPostgresContainer("")
	if err != nil {
		t.Skip(err)
	}
	got, err := DetectPostgresContainer("definitely-not-a-real-container-xyz")
	if err != nil {
		t.Fatal(err)
	}
	out, err := DetectPostgresContainer(got)
	if err != nil || out != got {
		t.Fatalf("fallback not running: got=%q running=%q err=%v", got, running, err)
	}
}

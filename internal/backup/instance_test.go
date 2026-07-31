package backup

import "testing"

func TestParseDatabaseURL(t *testing.T) {
	u, p, d, err := ParseDatabaseURL("postgres://goolify:secret@127.0.0.1:5432/goolify?sslmode=disable")
	if err != nil || u != "goolify" || p != "secret" || d != "goolify" {
		t.Fatalf("got %q %q %q %v", u, p, d, err)
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

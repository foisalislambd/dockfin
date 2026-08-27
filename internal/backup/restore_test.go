package backup

import "testing"

func TestRestoreDatabaseUnsupportedEngine(t *testing.T) {
	if err := RestoreDatabase(nil, "mongodb", "c", "p", "/tmp/x"); err == nil {
		t.Fatal("expected unsupported engine error")
	}
}

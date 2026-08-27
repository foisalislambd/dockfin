package backup

import (
	"strings"
	"testing"
)

func TestRestoreDatabaseUnsupportedEngine(t *testing.T) {
	if err := RestoreDatabase(nil, "mongodb", "c", "p", "/tmp/x"); err == nil {
		t.Fatal("expected unsupported engine error")
	}
}

func TestPostgresCustomFormat(t *testing.T) {
	if !PostgresCustomFormat("/data/dockfin/backups/postgresql-id-20260101.dump") {
		t.Fatal("expected .dump")
	}
	if !PostgresCustomFormat("x.BACKUP") {
		t.Fatal("expected .backup")
	}
	if PostgresCustomFormat("old.sql") {
		t.Fatal(".sql must keep psql restore")
	}
}

func TestDefaultFilenamePostgresCustom(t *testing.T) {
	name := DefaultFilename("postgresql", "abc")
	if !strings.HasSuffix(name, ".dump") {
		t.Fatalf("got %s", name)
	}
	if !strings.HasSuffix(DefaultFilename("mysql", "abc"), ".sql") {
		t.Fatal("mysql should stay .sql")
	}
}


package backup

import (
	"strings"
	"testing"
)

func TestRestoreDatabaseUnsupportedEngine(t *testing.T) {
	if err := RestoreDatabase(nil, "cassandra", "c", "p", "/tmp/x"); err == nil {
		t.Fatal("expected unsupported engine error")
	}
	if !DumpSupported("mongodb") || !DumpSupported("clickhouse") || !DumpSupported("dragonfly") {
		t.Fatal("expected mongo/clickhouse/dragonfly dump support")
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
	if !strings.HasSuffix(DefaultFilename("dragonfly", "abc"), ".tar.gz") {
		t.Fatal("dragonfly dumps are volume tarballs")
	}
}

func TestRestoreContainerDirCmdDoesNotQuoteInsideSh(t *testing.T) {
	cmd, err := restoreContainerDirCmd("dockfin-db-1", "/data/dockfin/backups/clickhouse-id-20260101-150405.tar.gz", "/var/lib/clickhouse")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd, `/backup/'`) {
		t.Fatalf("quoted filename leaked inside sh -c: %s", cmd)
	}
	if !strings.Contains(cmd, "tar xzf /backup/clickhouse-id-20260101-150405.tar.gz") {
		t.Fatalf("expected unquoted tar path in script, got %s", cmd)
	}
}

func TestDumpContainerDirCmdSafeLeaf(t *testing.T) {
	_, err := dumpContainerDirCmd("c", "/data", "/tmp/evil;rm.tar.gz")
	if err == nil {
		t.Fatal("expected invalid filename")
	}
}


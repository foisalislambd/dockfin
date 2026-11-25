package backup

import (
	"strings"
	"testing"
)

func TestDumpCommandsRejectTraversal(t *testing.T) {
	if _, err := dumpContainerDirCmd("web", "/etc", "/data/dockfin/backups/ok.tar.gz"); err == nil {
		t.Fatal("accepted /etc")
	}
	if _, err := dumpContainerDirCmd("web", "/data", "/tmp/../../etc/passwd"); err == nil {
		t.Fatal("accepted traversal filename")
	}
	cmd, err := dumpContainerDirCmd("web'; id", "/data", "/data/dockfin/backups/db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, `'web'"'"'; id'`) {
		t.Fatal(cmd)
	}
	if !DumpSupported("postgresql") || !DumpSupported("sqlite") || DumpSupported("nope") {
		t.Fatal("engine support")
	}
	if PostgresCustomFormat("dump.sql") || !PostgresCustomFormat("dump.dump") {
		t.Fatal("custom format")
	}
	if AppVolumeArchivePath("../x", "a.tar") == "" || strings.Contains(DefaultAppBackupFilename("app"), "..") {
		t.Fatal("archive path")
	}
}

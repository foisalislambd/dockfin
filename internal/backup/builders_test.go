package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShellQuoteStopsQuoteBreakout(t *testing.T) {
	if shellQuote("") != "''" {
		t.Fatalf("empty: %s", shellQuote(""))
	}
	got := shellQuote(`p'; rm -rf /; echo '`)
	if got != `'p'"'"'; rm -rf /; echo '"'"''` {
		t.Fatalf("quote breakout: %s", got)
	}
	cmd, err := dumpContainerDirCmd(`c';touch /tmp/pwned;'`, "/data", "/data/dockfin/backups/ok.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd, `c';touch`) {
		t.Fatalf("container name was not quoted: %s", cmd)
	}
	if !strings.Contains(cmd, "alpine:3.21") || !strings.Contains(cmd, "ok.tar.gz") {
		t.Fatalf("command missing archive pieces: %s", cmd)
	}
}

func TestDumpContainerDirCmdRejectsEscape(t *testing.T) {
	cases := []struct{ dir, out string }{
		{"/tmp", "/data/dockfin/backups/ok.tar.gz"},
		{"/data/../etc", "/data/dockfin/backups/ok.tar.gz"},
		{"/data", "/data/dockfin/backups/../etc/passwd"},
		{"/data", "/data/dockfin/backups/evil;rm.tar.gz"},
		{"/data", "/data/dockfin/backups/" + strings.Repeat("a", 201) + ".tar.gz"},
		{"/var/lib/clickhouse", ""},
	}
	for _, tc := range cases {
		if _, err := dumpContainerDirCmd("db", tc.dir, tc.out); err == nil {
			t.Fatalf("accepted dir=%q out=%q", tc.dir, tc.out)
		}
	}
	cmd, err := dumpContainerDirCmd("db", "/var/lib/sqlite", "/data/dockfin/backups/sqlite-1.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "sqlite-1.tar.gz") || !strings.Contains(cmd, "/var/lib/sqlite") {
		t.Fatalf("%s", cmd)
	}
	if _, err := restoreContainerDirCmd("db", "/data/dockfin/backups/../x.tar.gz", "/data"); err == nil {
		t.Fatal("restore accepted dot-dot path")
	}
	if _, err := restoreContainerDirCmd("db", "/data/dockfin/backups/ok.tar.gz", "/etc"); err == nil {
		t.Fatal("restore accepted data dir")
	}
}

func TestEngineSupportAndFilenameEdges(t *testing.T) {
	for _, engine := range []string{"postgresql", "mysql", "mariadb", "redis", "keydb", "dragonfly", "mongodb", "clickhouse", "sqlite"} {
		if !DumpSupported(engine) || !RestoreSupported(engine) {
			t.Fatalf("expected support for %s", engine)
		}
	}
	for _, engine := range []string{"", "Postgres", "oracle", "sqlite3"} {
		if DumpSupported(engine) || RestoreSupported(engine) {
			t.Fatalf("unexpected support for %q", engine)
		}
	}
	if err := DumpDatabase(nil, "oracle", "c", "p", "/tmp/x"); err == nil || !strings.Contains(err.Error(), "oracle") {
		t.Fatalf("dump: %v", err)
	}
	if !strings.HasSuffix(DefaultFilename("redis", "id"), ".rdb") || !strings.HasSuffix(DefaultFilename("keydb", "id"), ".rdb") {
		t.Fatal("redis family extension")
	}
	if !strings.HasSuffix(DefaultFilename("mongodb", "id"), ".archive") || !strings.HasSuffix(DefaultFilename("clickhouse", "id"), ".tar.gz") {
		t.Fatal("archive extensions")
	}
	if !PostgresCustomFormat("  X.PGDUMP  ") || PostgresCustomFormat("dump.sql.bak") || PostgresCustomFormat("") {
		t.Fatal("custom format suffix")
	}
	if filepathDir("leaf.sql") != "/data/dockfin/backups" || filepathBase("/a/b/c.sql") != "c.sql" {
		t.Fatal("path split")
	}
}

func TestArchivePathBuildersAndRetentionNoop(t *testing.T) {
	id := "1234567890abcdef"
	app := AppVolumeArchivePath(id, "app.tar.gz")
	if app != "/data/dockfin/backups/applications/"+id+"/app.tar.gz" {
		t.Fatal(app)
	}
	svc := ServiceVolumeArchivePath(id, "svc.tar.gz")
	if svc != "/data/dockfin/backups/services/"+id+"/svc.tar.gz" {
		t.Fatal(svc)
	}
	name := DefaultAppBackupFilename("short")
	if !strings.HasPrefix(name, "app-short-") || !strings.HasSuffix(name, ".tar.gz") {
		t.Fatalf("short id filename: %s", name)
	}
	long := DefaultAppBackupFilename(id)
	if !strings.HasPrefix(long, "app-12345678-") {
		t.Fatalf("long id filename: %s", long)
	}
	svcName := DefaultServiceBackupFilename("abc")
	if !strings.HasPrefix(svcName, "svc-abc-") || !strings.HasSuffix(svcName, ".tar.gz") {
		t.Fatalf("service filename: %s", svcName)
	}
	if err := TarHostPaths(nil, "/tmp/out.tar.gz", nil); err == nil {
		t.Fatal("expected empty path list")
	}
	if err := TarHostPaths(nil, "/tmp/out.tar.gz", []string{"", "  "}); err == nil {
		t.Fatal("expected blank paths to be rejected")
	}
	if err := UntarHostPaths(nil, "  "); err == nil {
		t.Fatal("expected archive path")
	}
	if _, err := ListDockerImages(nil, " "); err == nil {
		t.Fatal("expected prefix")
	}
	if err := EnforceRemoteRetention(nil, "postgresql", "id", 3); err != nil {
		t.Fatal(err)
	}
	if err := EnforceRemoteRetention(nil, "postgresql", "id", 0); err != nil {
		t.Fatal(err)
	}
	if err := EnforceAppBackupRetention(nil, id, 2); err != nil {
		t.Fatal(err)
	}
	if err := EnforceServiceBackupRetention(nil, "", 2); err != nil {
		t.Fatal(err)
	}
	if err := UploadRemoteToS3(nil, "/data/dockfin/backups/a.sql", "a.sql", S3Creds{Bucket: "b"}); err == nil {
		t.Fatal("expected incomplete credentials")
	}
}

func TestParseDatabaseURLEdges(t *testing.T) {
	u, p, d, err := ParseDatabaseURL("postgres://localhost/mydb")
	if err != nil || u != "dockfin" || p != "" || d != "mydb" {
		t.Fatalf("%q %q %q %v", u, p, d, err)
	}
	u, p, d, err = ParseDatabaseURL("postgres://user:p%40ss@localhost:5432/app?sslmode=disable")
	if err != nil || u != "user" || p != "p@ss" || d != "app" {
		t.Fatalf("%q %q %q %v", u, p, d, err)
	}
	_, _, d, err = ParseDatabaseURL("postgres://localhost")
	if err != nil || d != "dockfin" {
		t.Fatalf("default db %q %v", d, err)
	}
	if _, _, _, err := ParseDatabaseURL(":"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestRestoreInstanceLocalRejectsNames(t *testing.T) {
	if err := RestoreInstanceLocal(t.TempDir(), "", "u", "p", "db", "pg-dump-dockfin-1.sql"); err == nil {
		t.Fatal("expected container")
	}
	dir := t.TempDir()
	for _, name := range []string{"", "../pg-dump-dockfin-1.sql", "pg-dump-dockfin-1.sql/x", "notes.sql", "pg-dump-dockfin-1.dump"} {
		if err := RestoreInstanceLocal(dir, "postgres", "u", "p", "db", name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}

func TestEnforceLocalRetention(t *testing.T) {
	dir := t.TempDir()
	if err := EnforceLocalRetention(filepath.Join(dir, "missing"), 1); err != nil {
		t.Fatal(err)
	}
	dumpDir := InstanceDumpDir(dir)
	if err := os.MkdirAll(dumpDir, 0o750); err != nil {
		t.Fatal(err)
	}
	keepNote := filepath.Join(dumpDir, "notes.txt")
	if err := os.WriteFile(keepNote, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dumpDir, "pg-dump-dockfin-dir.sql"), 0o750); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	var names []string
	for i := 0; i < 4; i++ {
		name := "pg-dump-dockfin-20060102-15040" + string(rune('0'+i)) + ".sql"
		names = append(names, name)
		path := filepath.Join(dumpDir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(time.Duration(i)*time.Minute), now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnforceLocalRetention(dir, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dumpDir, names[0])); err != nil {
		t.Fatal("unlimited retention deleted a dump")
	}
	if err := EnforceLocalRetention(dir, 2); err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		_, err := os.Stat(filepath.Join(dumpDir, name))
		if i >= 2 && err != nil {
			t.Fatalf("newest %s removed: %v", name, err)
		}
		if i < 2 && err == nil {
			t.Fatalf("oldest %s kept", name)
		}
	}
	if _, err := os.Stat(keepNote); err != nil {
		t.Fatal("unrelated file removed")
	}
	if _, err := os.Stat(filepath.Join(dumpDir, "pg-dump-dockfin-dir.sql")); err != nil {
		t.Fatal("directory removed")
	}
}

func TestRedactS3MessageTruncates(t *testing.T) {
	msg := "https://key:secret@host/bucket " + strings.Repeat("e", 600)
	got := RedactS3Message(msg)
	if strings.Contains(got, "secret") || len(got) > 500 {
		t.Fatalf("len=%d %q", len(got), got[:40])
	}
}

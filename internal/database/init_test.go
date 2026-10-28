package database

import (
	"strings"
	"testing"
)

func TestInitDirStaysOutsideData(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	dir := InitDir(id)
	data := HostDataDir(id)
	ssl := sslDir(id)
	if dir == data || strings.HasPrefix(dir, data+"/") || strings.HasPrefix(data, dir+"/") {
		t.Fatalf("init dir %s overlaps data dir %s", dir, data)
	}
	if dir == ssl || strings.HasPrefix(dir, ssl+"/") {
		t.Fatalf("init dir %s overlaps ssl dir %s", dir, ssl)
	}
	args := InitVolumeArgs(id, []InitScript{{Filename: "01.sql"}})
	if len(args) != 2 || !strings.Contains(args[1], dir+":/docker-entrypoint-initdb.d:ro") {
		t.Fatalf("volume %v", args)
	}
	if InitVolumeArgs(id, nil) != nil {
		t.Fatal("empty scripts should not mount")
	}
}

func TestNormalizeInitScripts(t *testing.T) {
	ok, err := NormalizeInitScripts("postgresql", []InitScript{{Filename: "01_schema.sql", Content: "SELECT 1;\n"}})
	if err != nil || len(ok) != 1 || ok[0].Filename != "01_schema.sql" {
		t.Fatalf("%v %v", ok, err)
	}
	if _, err := NormalizeInitScripts("postgresql", []InitScript{{Filename: "../etc/cron.d/pwn.sql", Content: "x"}}); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if _, err := NormalizeInitScripts("postgresql", []InitScript{{Filename: "init;rm.sql", Content: "x"}}); err == nil {
		t.Fatal("shell metacharacter was accepted")
	}
	if _, err := NormalizeInitScripts("postgresql", []InitScript{{Filename: "setup.js", Content: "x"}}); err == nil {
		t.Fatal("javascript was accepted for postgresql")
	}
	if _, err := NormalizeInitScripts("mongodb", []InitScript{{Filename: "seed.js", Content: "db.x.insertOne({})"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeInitScripts("mongodb", []InitScript{{Filename: "seed.sql", Content: "x"}}); err == nil {
		t.Fatal("sql was accepted for mongodb")
	}
	if _, err := NormalizeInitScripts("redis", []InitScript{{Filename: "a.sh", Content: "echo"}}); err == nil {
		t.Fatal("redis accepted an init script")
	}
	if _, err := NormalizeInitScripts("postgresql", []InitScript{
		{Filename: "a.sql", Content: "1"},
		{Filename: "a.sql", Content: "2"},
	}); err == nil {
		t.Fatal("duplicate filename was accepted")
	}
	if _, err := NormalizeInitScripts("postgresql", []InitScript{{Filename: "a.sql", Content: "a\x00b"}}); err == nil {
		t.Fatal("binary content was accepted")
	}
	if !ReservesInitDir("/docker-entrypoint-initdb.d/init.sql") {
		t.Fatal("a file inside the init directory should be reserved")
	}
	if ReservesInitDir("/etc/postgresql") {
		t.Fatal("an unrelated path was reserved")
	}
}

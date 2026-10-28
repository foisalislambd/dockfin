package database

import "testing"

func TestNormalizeSqliteDatabases(t *testing.T) {
	got, err := NormalizeSqliteDatabases(" app.db, jobs.sqlite, app.db ")
	if err != nil || got != "app.db,jobs.sqlite" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := NormalizeSqliteDatabases("../x.db"); err == nil {
		t.Fatal("path was accepted")
	}
	if _, err := NormalizeSqliteDatabases(".hidden.db"); err == nil {
		t.Fatal("hidden file was accepted")
	}
	if _, err := NormalizeSqliteDatabases("app.db,"); err == nil {
		t.Fatal("trailing comma was accepted")
	}
	if _, err := NormalizeSqliteDatabases("a,b,c,d,e,f,g,h,i"); err == nil {
		t.Fatal("ninth file was accepted")
	}
	if got, err := NormalizeSqliteDatabases(""); err != nil || got != "database.sqlite" {
		t.Fatalf("default %q %v", got, err)
	}
}

func TestSqliteDataDirID(t *testing.T) {
	id, ok := SqliteDataDirID("/data/dockfin/databases/abc")
	if !ok || id != "abc" {
		t.Fatalf("%q %v", id, ok)
	}
	if _, ok := SqliteDataDirID("/data/dockfin/databases/abc/base"); ok {
		t.Fatal("child path was accepted")
	}
}

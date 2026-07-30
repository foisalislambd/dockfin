package httpapi

import "testing"

func TestSafeBackupFilename(t *testing.T) {
	ok := []string{
		"postgresql-abc-20260101-120000.sql",
		"db_backup.sql",
		"a.b-c_1.sql",
	}
	for _, s := range ok {
		if !safeBackupFilename(s) {
			t.Fatalf("expected safe: %q", s)
		}
	}
	bad := []string{
		"",
		"../etc/passwd",
		"foo/bar.sql",
		"foo\\bar.sql",
		"foo;rm.sql",
		"..",
		"a..b.sql", // contains ".."
	}
	for _, s := range bad {
		if safeBackupFilename(s) {
			t.Fatalf("expected unsafe: %q", s)
		}
	}
}

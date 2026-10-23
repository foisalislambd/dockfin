package httpapi

import "testing"

func TestParseDockerImageLines(t *testing.T) {
	out := "lissy93/dashy\tlatest\t4de769e76043\t547MB\t22 hours ago\n" +
		"\n" +
		"broken-line\n" +
		"postgres\t16-alpine\tf46107955874\t420MB\t34 hours ago\textra\n"
	rows, truncated := parseDockerImageLines(out, 10)
	if truncated {
		t.Fatal("expected no truncation")
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Repository != "lissy93/dashy" || rows[0].Tag != "latest" || rows[0].Size != "547MB" || rows[0].Created != "22 hours ago" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].Created != "34 hours ago\textra" {
		t.Fatalf("created field split too far: %q", rows[1].Created)
	}

	rows, truncated = parseDockerImageLines(out, 1)
	if !truncated || len(rows) != 1 {
		t.Fatalf("limit: rows=%d truncated=%v", len(rows), truncated)
	}

	rows, truncated = parseDockerImageLines("", 5)
	if truncated || len(rows) != 0 {
		t.Fatalf("empty: rows=%d truncated=%v", len(rows), truncated)
	}
}

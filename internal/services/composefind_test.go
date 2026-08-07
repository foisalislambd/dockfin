package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindComposeFilesPrefersRootYaml(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docker-compose.yml"), "services: {}\n")
	mustWrite(t, filepath.Join(dir, "docker-compose.yaml"), "services: {}\n")
	sub := filepath.Join(dir, "deploy")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sub, "compose.yaml"), "services: {}\n")

	got, err := FindComposeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if PreferComposeFile(got) != "/docker-compose.yaml" {
		t.Fatalf("prefer got %q from %#v", PreferComposeFile(got), got)
	}
	if len(got) < 3 {
		t.Fatalf("expected multiple candidates, got %#v", got)
	}
}

func TestFindComposeFilesNestedOnly(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "apps", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sub, "compose.yml"), "services: {}\n")

	got, err := FindComposeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "/apps/api/compose.yml"
	if PreferComposeFile(got) != want {
		t.Fatalf("got %q want %q (%#v)", PreferComposeFile(got), want, got)
	}
}

func TestFindComposeFilesSkipsTooDeep(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(deep, "compose.yaml"), "services: {}\n")
	mustWrite(t, filepath.Join(dir, "docker-compose.yml"), "services: {}\n")

	got, err := FindComposeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if PreferComposeFile(got) != "/docker-compose.yml" {
		t.Fatalf("got %#v", got)
	}
	for _, p := range got {
		if strings.Contains(p, "/a/b/c/") {
			t.Fatalf("too-deep path should be skipped: %#v", got)
		}
	}
}

func TestFindComposeFilesSkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(bad, "docker-compose.yml"), "services: {}\n")
	mustWrite(t, filepath.Join(dir, "compose.yaml"), "services: {}\n")

	got, err := FindComposeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if PreferComposeFile(got) != "/compose.yaml" {
		t.Fatalf("got %#v", got)
	}
	for _, p := range got {
		if strings.Contains(p, "node_modules") {
			t.Fatalf("should skip node_modules: %#v", got)
		}
	}
}

func TestNormalizeComposeLocation(t *testing.T) {
	if got := NormalizeComposeLocation("auto"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeComposeLocation("docker-compose.yml"); got != "/docker-compose.yml" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeComposeLocation("../etc/passwd"); got != "" {
		t.Fatalf("expected empty for traversal, got %q", got)
	}
	if got := NormalizeComposeLocation("/ok/../evil.yml"); got != "" {
		t.Fatalf("expected empty for cleaned traversal, got %q", got)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

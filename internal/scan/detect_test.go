package scan

import "testing"

func TestDetectComposeBeforeDockerfile(t *testing.T) {
	got := Detect([]string{"Dockerfile", "docker-compose.yml", "package.json"})
	if got.BuildPack != "dockercompose" || got.ComposeFile != "docker-compose.yml" {
		t.Fatalf("%+v", got)
	}
}

func TestDetectRailpack(t *testing.T) {
	got := Detect([]string{"src/main.go", "go.mod"})
	if got.BuildPack != "railpack" {
		t.Fatalf("%+v", got)
	}
}

func TestDetectStatic(t *testing.T) {
	got := Detect([]string{"index.html", "app.css"})
	if got.BuildPack != "static" {
		t.Fatalf("%+v", got)
	}
}

func TestDetectIgnoresNestedCompose(t *testing.T) {
	got := Detect([]string{"docs/docker-compose.yml", "Dockerfile"})
	if got.BuildPack != "dockerfile" {
		t.Fatalf("%+v", got)
	}
}

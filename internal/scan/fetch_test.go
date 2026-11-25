package scan

import (
	"context"
	"strings"
	"testing"
)

func TestParseRepoAcceptsForms(t *testing.T) {
	cases := []struct {
		raw, host, owner, name string
	}{
		{" Dockfin/Dockfin ", "github.com", "Dockfin", "Dockfin"},
		{"https://github.com/dockfin/dockfin.git", "github.com", "dockfin", "dockfin"},
		{"git@github.com:dockfin/dockfin.git", "github.com", "dockfin", "dockfin"},
		{"ssh://git@github.com/dockfin/dockfin.git", "github.com", "dockfin", "dockfin"},
		{"https://gitlab.example.com/group/proj.git", "gitlab.example.com", "group", "proj"},
		{"https://github.com/dockfin/dockfin/tree/main", "github.com", "dockfin", "dockfin"},
	}
	for _, tc := range cases {
		got, err := ParseRepo(tc.raw)
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if got.Host != tc.host || got.Owner != tc.owner || got.Name != tc.name {
			t.Fatalf("%s: %+v", tc.raw, got)
		}
	}
}

func TestParseRepoRejects(t *testing.T) {
	for _, raw := range []string{
		"",
		"   ",
		"only-owner",
		"owner/",
		"/name",
		"owner/name/extra",
		"https://github.com",
		"https://github.com/onlyone",
		"://",
		"not a repo",
	} {
		if _, err := ParseRepo(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestListPathsRejectsUnsafeHosts(t *testing.T) {
	ctx := context.Background()
	for _, host := range []string{"", "127.0.0.1", "localhost", "169.254.169.254", "metadata.google.internal", "10.1.2.3"} {
		_, err := ListPaths(ctx, Repo{Host: host, Owner: "o", Name: "n"}, "main", "token")
		if err == nil {
			t.Fatalf("accepted host %q", host)
		}
	}
}

func TestDetectEdges(t *testing.T) {
	nested := Detect([]string{`pkg\go.mod`, `..\Dockerfile`, `a/b/Dockerfile`})
	if nested.BuildPack != "dockerfile" || nested.Dockerfile != "" {
		t.Fatalf("fallback: %+v", nested)
	}
	if len(nested.Reasons) == 0 {
		t.Fatal("expected a reason")
	}
	prefer := Detect([]string{"deploy/Dockerfile", "Dockerfile"})
	if prefer.BuildPack != "dockerfile" || prefer.Dockerfile != "Dockerfile" {
		t.Fatalf("%+v", prefer)
	}
	compose := Detect([]string{"compose.yaml", "index.html"})
	if compose.BuildPack != "dockercompose" || compose.ComposeFile != "compose.yaml" {
		t.Fatalf("%+v", compose)
	}
	mod := Detect([]string{"GO.MOD"})
	if mod.BuildPack != "railpack" || !strings.Contains(mod.Reasons[0], "GO.MOD") {
		t.Fatalf("%+v", mod)
	}
	subIndex := Detect([]string{"site/index.html"})
	if subIndex.BuildPack != "dockerfile" {
		t.Fatalf("%+v", subIndex)
	}
	empty := Detect(nil)
	if empty.BuildPack != "dockerfile" {
		t.Fatalf("%+v", empty)
	}
}

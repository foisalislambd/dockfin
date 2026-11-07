package imagescan

import "testing"

func TestSelectTagsPatchLineAndMajor(t *testing.T) {
	got := SelectTags("1.27.1", []string{"1.27.1", "1.27.4", "1.28.0", "2.0.0", "1.27.1-alpine", "1.27.5-rc1"})
	if got.Patch != "1.27.4" || got.Line != "1.28.0" || got.Major != "2.0.0" || got.Floating {
		t.Fatalf("%+v", got)
	}
}

func TestSelectTagsIgnoresLatest(t *testing.T) {
	got := SelectTags("latest", []string{"1.0.0", "latest"})
	if got.Patch != "" || got.Line != "" || got.Major != "" {
		t.Fatalf("%+v", got)
	}
}

func TestSelectTagsKeepsSuffix(t *testing.T) {
	got := SelectTags("1.27.1-alpine", []string{"1.27.2", "1.27.3-alpine", "1.28.0-alpine"})
	if got.Patch != "1.27.3-alpine" || got.Line != "1.28.0-alpine" {
		t.Fatalf("%+v", got)
	}
}

func TestSelectTagsFloating(t *testing.T) {
	got := SelectTags("15-alpine", []string{"15-alpine", "15.4-alpine", "16-alpine"})
	if !got.Floating || got.Line != "15-alpine" || got.Patch != "" || got.Major != "16-alpine" {
		t.Fatalf("%+v", got)
	}
}

func TestImageName(t *testing.T) {
	repo, tag, ok := ImageName("nginx:1.27.1")
	if !ok || repo != "docker.io/library/nginx" || tag != "1.27.1" {
		t.Fatalf("%s %s %v", repo, tag, ok)
	}
	repo, tag, ok = ImageName("ghcr.io/org/app:1.2.3")
	if !ok || repo != "ghcr.io/org/app" || tag != "1.2.3" {
		t.Fatalf("%s %s %v", repo, tag, ok)
	}
}

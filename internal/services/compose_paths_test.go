package services

import "testing"

func TestIsDatabaseImage(t *testing.T) {
	cases := []struct {
		img  string
		want bool
	}{
		{"postgres:16", true},
		{"docker.io/library/mysql:8", true},
		{"redis", true},
		{"ghcr.io/myorg/web:latest", false},
		{"nginx:alpine", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsDatabaseImage(c.img); got != c.want {
			t.Fatalf("IsDatabaseImage(%q)=%v want %v", c.img, got, c.want)
		}
	}
}

func TestJoinBaseAndComposePath(t *testing.T) {
	if got := JoinBaseAndComposePath("/", "/docker-compose.yaml"); got != "/docker-compose.yaml" {
		t.Fatalf("got %q", got)
	}
	if got := JoinBaseAndComposePath("/apps", "/docker-compose.yaml"); got != "/apps/docker-compose.yaml" {
		t.Fatalf("got %q", got)
	}
	if got := JoinBaseAndComposePath("apps/api", "compose.yml"); got != "/apps/api/compose.yml" {
		t.Fatalf("got %q", got)
	}
}

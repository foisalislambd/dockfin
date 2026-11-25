package terminal

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidContainerNameBounds(t *testing.T) {
	ok := []string{"a", "A1._-", "app_web-1", strings.Repeat("a", 128)}
	for _, name := range ok {
		if !ValidContainerName(name) {
			t.Fatalf("rejected %q", name)
		}
	}
	bad := []string{
		"",
		strings.Repeat("a", 129),
		".hidden",
		"-flag",
		"app;id",
		"app|id",
		"app&id",
		"app`id`",
		"app$(id)",
		"app name",
		"../app",
		"app\nid",
		"app\x00id",
		"/bin/sh",
	}
	for _, name := range bad {
		if ValidContainerName(name) {
			t.Fatalf("accepted %q", name)
		}
		if _, err := DockerExec(name); err == nil {
			t.Fatalf("exec accepted %q", name)
		}
		if _, err := DockerAttach(name); err == nil {
			t.Fatalf("attach accepted %q", name)
		}
	}
}

func TestDockerCommandsAreFixed(t *testing.T) {
	exec, err := DockerExec("web_1")
	if err != nil || exec != "docker exec -it web_1 sh" {
		t.Fatalf("%q %v", exec, err)
	}
	attach, err := DockerAttach("web_1")
	if err != nil {
		t.Fatal(err)
	}
	want := "docker logs --tail 100 web_1 2>&1; exec docker attach --detach-keys=ctrl-p,ctrl-q --sig-proxy=false web_1"
	if attach != want {
		t.Fatalf("%s", attach)
	}
	if strings.Count(attach, "web_1") != 2 {
		t.Fatal(attach)
	}
}

func TestManagerRejectsBadContainerBeforeDial(t *testing.T) {
	m := NewManager(nil, nil)
	if m.Logger == nil || m.sessions == nil {
		t.Fatal("manager not initialized")
	}
	id, err := m.Create(CreateOpts{Container: "app;id", TeamID: uuid.New()})
	if err == nil || id != uuid.Nil {
		t.Fatalf("id=%s err=%v", id, err)
	}
	if _, err := m.Get(uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected missing session")
	}
	m.Close(uuid.New())
}

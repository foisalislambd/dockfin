package terminal

import (
	"strings"
	"testing"
)

func TestDockerExecRejectsInjection(t *testing.T) {
	got, err := DockerExec("app_web-1")
	if err != nil || got != "docker exec -it app_web-1 sh" {
		t.Fatalf("%q %v", got, err)
	}
	for _, name := range []string{"", "app;id", "app\nid", "-name", "app name", "../app", "$(id)"} {
		if _, err := DockerExec(name); err == nil {
			t.Fatalf("accepted %q", name)
		}
		if _, err := DockerAttach(name); err == nil {
			t.Fatalf("attach accepted %q", name)
		}
	}
	attach, err := DockerAttach("app")
	if err != nil || !strings.Contains(attach, "docker attach") || !strings.Contains(attach, " app") {
		t.Fatal(attach, err)
	}
}

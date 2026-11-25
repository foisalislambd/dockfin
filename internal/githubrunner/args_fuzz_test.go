package githubrunner

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func FuzzStartArgs(f *testing.F) {
	f.Add("ghcr.io/actions/actions-runner:latest", "host", "/data/dockfin/github-runners/env", "/home/runner/hook.sh", "1", "512m")
	f.Add("--privileged", "dind", "-e EVIL=1", "/tmp/../hook.sh", "999", "1b")
	f.Add("nginx:alpine\n", "host", "/tmp/env", "/tmp/hook.sh", "0.5", "256m")
	f.Fuzz(func(t *testing.T, image, mode, envFile, hook, cpu, mem string) {
		args, err := StartArgs(uuid.Nil, image, mode, envFile, hook, cpu, mem)
		if err != nil {
			return
		}
		if len(args) == 0 || args[0] != "docker" {
			t.Fatalf("args %v", args)
		}
		for _, a := range args {
			if strings.ContainsAny(a, "\r\n\x00") || strings.Contains(a, "..") {
				t.Fatalf("unsafe arg %q in %v", a, args)
			}
		}
		if strings.HasPrefix(strings.TrimSpace(image), "-") || strings.Contains(strings.TrimSpace(image), "\n") {
			t.Fatalf("hostile image survived: %q args %v", image, args)
		}
	})
}

func TestDockerArgsRejectInjection(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cases := []struct{ image, mode, env, hook, cpu, mem string }{
		{"--privileged", "host", "/tmp/env", "/tmp/hook.sh", "", ""},
		{"nginx:alpine", "host", "-e EVIL=1", "/tmp/hook.sh", "", ""},
		{"nginx:alpine", "host", "/tmp/env", "/tmp/../hook.sh", "", ""},
		{"nginx:alpine", "dind", "/tmp/env", "/tmp/hook.sh", "1;id", "256m"},
		{"nginx:alpine", "host", "/tmp/env", "/tmp/hook.sh", "1", "256m;id"},
		{"nginx\nalpine", "host", "/tmp/env", "/tmp/hook.sh", "", ""},
	}
	for _, c := range cases {
		if _, err := StartArgs(id, c.image, c.mode, c.env, c.hook, c.cpu, c.mem); err == nil {
			t.Fatalf("accepted %+v", c)
		}
		if _, err := PrepareArgs(id, c.image, c.mode, c.cpu, c.mem); err == nil && (strings.ContainsAny(c.image, "\n") || strings.HasPrefix(c.image, "-") || strings.Contains(c.cpu, ";") || strings.Contains(c.mem, ";")) {
			t.Fatalf("prepare accepted %+v", c)
		}
	}
}

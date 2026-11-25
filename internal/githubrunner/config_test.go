package githubrunner

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMatchesNeedsACustomLabel(t *testing.T) {
	custom := []string{"dockfin"}
	if !Matches(custom, []string{"self-hosted", "linux", "dockfin"}) {
		t.Fatal("matching job was refused")
	}
	if Matches(custom, []string{"self-hosted", "linux"}) {
		t.Fatal("a generic job was accepted")
	}
	if Matches(custom, []string{"self-hosted", "dockfin", "gpu"}) {
		t.Fatal("an unknown label was accepted")
	}
}

func TestNormalizeLabelsRejectsGenericOnly(t *testing.T) {
	if _, err := NormalizeLabels("self-hosted, linux"); err == nil {
		t.Fatal("generic labels were accepted as the custom label")
	}
	got, err := NormalizeLabels("Dockfin, self-hosted")
	if err != nil || len(got) != 1 || got[0] != "dockfin" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestStartCommandDoesNotMountTheHostSocket(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	for _, mode := range []string{"none", "dind"} {
		args, err := StartArgs(id, "", mode, EnvPath(id), HookFile(id), "1", "1g")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "/var/run/docker.sock:/") || strings.Contains(joined, "encoded_jit") {
			t.Fatalf("%s command exposes the host or the secret: %s", mode, joined)
		}
		if !strings.Contains(joined, "--env-file "+EnvPath(id)) {
			t.Fatalf("%s missing env file", mode)
		}
	}
	cmds, err := PrepareArgs(id, "", "dind", "1", "1g")
	if err != nil {
		t.Fatal(err)
	}
	prepare := strings.Join(flatten(cmds), " ")
	if !strings.Contains(prepare, "--privileged") || strings.Contains(prepare, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("sidecar: %s", prepare)
	}
}

func TestHookRefusesPullRequests(t *testing.T) {
	script := HookScript()
	if !strings.Contains(script, "pull_request") || !IsPullRequestEvent("pull_request_target") {
		t.Fatal(script)
	}
	if IsPullRequestEvent("push") {
		t.Fatal("push was treated as a pull request")
	}
}

func TestStartArgsRejectsFlagInjection(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if _, err := StartArgs(id, "--privileged", "none", EnvPath(id), "", "1", "1g"); err == nil {
		t.Fatal("image flag was accepted")
	}
	if _, err := StartArgs(id, "", "none", "-e EVIL=1", "", "1", "1g"); err == nil {
		t.Fatal("env file flag was accepted")
	}
	if _, err := StartArgs(id, "", "none", EnvPath(id), "/tmp/../hook.sh", "1", "1g"); err == nil {
		t.Fatal("hook path traversal was accepted")
	}
	if _, err := PrepareArgs(id, "alpine\n-v /:/host", "none", "1", "1g"); err == nil {
		t.Fatal("image newline was accepted")
	}
}

func flatten(cmds [][]string) []string {
	var out []string
	for _, cmd := range cmds {
		out = append(out, cmd...)
	}
	return out
}

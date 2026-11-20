package httpapi

import (
	"context"
	"os"
	"os/exec"

	"github.com/dockfin/dockfin/internal/git"
)

// runLocalGitClone clones on the control plane. The remote and branch are
// validated first so a repository value cannot be a git option or ext:: helper.
func runLocalGitClone(ctx context.Context, branch, repo, dest string, extraEnv []string) ([]byte, error) {
	if err := git.ValidateRemote(repo); err != nil {
		return nil, err
	}
	if err := git.ValidateRef(branch); err != nil {
		return nil, err
	}
	args := []string{"-c", "protocol.file.allow=never", "-c", "protocol.ext.allow=never", "clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, "--", repo, dest)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	return cmd.CombinedOutput()
}

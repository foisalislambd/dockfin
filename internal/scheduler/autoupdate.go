package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// defaultUpdateImage is the control-plane image repository (without tag).
const defaultUpdateImage = "ghcr.io/foisalislambd/dockfin"

// dockerSocketPath is the host socket that must be mounted into the control-plane
// container for self-update to work.
const dockerSocketPath = "/var/run/docker.sock"

// updateTagForChannel maps the instance update channel to an image tag.
func updateTagForChannel(channel string) string {
	switch strings.TrimSpace(strings.ToLower(channel)) {
	case "next":
		return "next"
	case "nightly":
		return "nightly"
	default:
		return "latest"
	}
}

// updateImageRef builds the fully-qualified image reference for a channel.
// registry is the instance docker_registry_url (docker.io or ghcr.io).
func updateImageRef(registry, channel string) string {
	repo := defaultUpdateImage
	if host := strings.TrimSpace(registry); host != "" && host != "ghcr.io" {
		repo = strings.TrimSuffix(host, "/") + "/foisalislambd/dockfin"
	}
	return repo + ":" + updateTagForChannel(channel)
}

// autoUpdateDir is the compose project directory on the host.
func autoUpdateDir() string {
	if d := strings.TrimSpace(os.Getenv("DOCKFIN_DIR")); d != "" {
		return d
	}
	return "/data/dockfin"
}

// autoUpdateAllowed requires a usable docker socket, or an explicit opt-in for
// environments where the socket lives elsewhere.
func autoUpdateAllowed() bool {
	if os.Getenv("DOCKFIN_AUTO_UPDATE") == "1" {
		return true
	}
	_, err := os.Stat(dockerSocketPath)
	return err == nil
}

var composeImageLine = regexp.MustCompile(`(?m)^(\s*image:\s*)(?:["']?)([^\s"']*dockfin[^\s"']*)(?:["']?)(\s*)$`)

// rewriteComposeImage points the dockfin service at the target image reference.
// Returns the new content and whether anything changed.
func rewriteComposeImage(content, image string) (string, bool) {
	changed := false
	out := composeImageLine.ReplaceAllStringFunc(content, func(m string) string {
		parts := composeImageLine.FindStringSubmatch(m)
		if len(parts) != 4 {
			return m
		}
		// Skip postgres/redis and other non-control-plane images.
		if !strings.Contains(parts[2], "dockfin") || strings.Contains(parts[2], "postgres") {
			return m
		}
		if parts[2] == image {
			return m
		}
		changed = true
		return parts[1] + image + parts[3]
	})
	return out, changed
}

func (r *Runner) runAutoUpdate(ctx context.Context, minute time.Time) {
	st, err := r.Store.GetInstanceSettings(ctx)
	if err != nil || !st.IsAutoUpdateEnabled {
		return
	}
	if !Matches(st.AutoUpdateFrequency, minute) {
		return
	}
	ran, err := r.Store.AutoUpdateRanThisMinute(ctx, minute)
	if err != nil || ran {
		return
	}
	if !autoUpdateAllowed() {
		_ = r.Store.SetAutoUpdateStatus(ctx, "skipped",
			"docker socket not available in the control plane — mount /var/run/docker.sock or set DOCKFIN_AUTO_UPDATE=1")
		return
	}
	image := updateImageRef(st.DockerRegistryURL, st.UpdateChannel)
	// Claim the minute before the (slow) pull so a later tick cannot double-run.
	_ = r.Store.SetAutoUpdateStatus(ctx, "running", "updating to "+image)
	go func() {
		bg := context.Background()
		status, msg := r.performAutoUpdate(bg, image)
		_ = r.Store.SetAutoUpdateStatus(bg, status, msg)
		r.Logger.Info("auto-update finished", "image", image, "status", status)
	}()
}

func (r *Runner) performAutoUpdate(ctx context.Context, image string) (string, string) {
	dir := autoUpdateDir()
	composePath := dir + "/docker-compose.yml"
	if content, err := os.ReadFile(composePath); err == nil {
		if next, changed := rewriteComposeImage(string(content), image); changed {
			if err := os.WriteFile(composePath, []byte(next), 0o644); err != nil {
				return "failed", fmt.Sprintf("rewrite compose image: %v", err)
			}
		}
	}
	var log strings.Builder
	out, err := runDockerCommand(ctx, dir, "compose", "pull")
	log.WriteString(strings.TrimSpace(out) + "\n")
	if err != nil {
		return "failed", fmt.Sprintf("docker compose pull: %v\n%s", err, trimLog(log.String()))
	}
	// `up -d` recreates this very container, so the process is usually killed
	// before it returns. Checkpoint the status first so the post-restart UI
	// shows the recreate rather than a phantom failure.
	_ = r.Store.SetAutoUpdateStatus(ctx, "running", trimLog("pulled "+image+", recreating containers\n"+log.String()))
	out, err = runDockerCommand(ctx, dir, "compose", "up", "-d")
	log.WriteString(strings.TrimSpace(out) + "\n")
	if err != nil {
		return "failed", fmt.Sprintf("docker compose up -d: %v\n%s", err, trimLog(log.String()))
	}
	return "finished", trimLog("updated to "+image+"\n"+log.String())
}

func runDockerCommand(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func trimLog(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 4000 {
		return s[len(s)-4000:]
	}
	return s
}

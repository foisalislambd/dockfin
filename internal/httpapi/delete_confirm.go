package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// DeleteOptions are Coolify-like danger-zone options sent with DELETE bodies.
type DeleteOptions struct {
	Password             string `json:"password"`
	ConfirmationName     string `json:"confirmation_name"`
	DeleteVolumes        *bool  `json:"delete_volumes"`
	DeleteConfigurations *bool  `json:"delete_configurations"`
	DeleteNetworks       *bool  `json:"delete_networks"`
	DockerCleanup        *bool  `json:"docker_cleanup"`
}

func (o DeleteOptions) volumes() bool {
	if o.DeleteVolumes == nil {
		return true
	}
	return *o.DeleteVolumes
}

func (o DeleteOptions) configurations() bool {
	if o.DeleteConfigurations == nil {
		return true
	}
	return *o.DeleteConfigurations
}

func (o DeleteOptions) networks() bool {
	if o.DeleteNetworks == nil {
		return true
	}
	return *o.DeleteNetworks
}

func (o DeleteOptions) dockerCleanup() bool {
	if o.DockerCleanup == nil {
		return true
	}
	return *o.DockerCleanup
}

func parseDeleteOptions(r *http.Request) DeleteOptions {
	var opts DeleteOptions
	if r.Body == nil {
		return opts
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(b) == 0 {
		return opts
	}
	_ = json.Unmarshal(b, &opts)
	return opts
}

// authorizeDestructiveAction enforces Coolify-style name (+ optional password) confirmation.
// requirePassword=false for empty project/environment deletes (Coolify parity).
func (a *API) authorizeDestructiveAction(w http.ResponseWriter, r *http.Request, expectedName string, requirePassword bool) (DeleteOptions, bool) {
	opts := parseDeleteOptions(r)
	skipTwoStep := false
	if cfg, err := a.Store.GetInstanceSettings(r.Context()); err == nil {
		skipTwoStep = cfg.DisableTwoStepConfirmation
	}

	if !skipTwoStep {
		if strings.TrimSpace(opts.ConfirmationName) != expectedName {
			writeError(w, http.StatusBadRequest, "confirmation name does not match")
			return opts, false
		}
		if requirePassword {
			user := currentUser(r)
			hash, err := a.Store.GetUserPasswordHash(r.Context(), user.ID)
			if err != nil {
				mapStoreErr(w, err)
				return opts, false
			}
			// Coolify: OAuth / no-password users skip password confirmation.
			if strings.TrimSpace(hash) != "" && !crypto.VerifyPassword(hash, opts.Password) {
				writeError(w, http.StatusForbidden, "invalid password")
				return opts, false
			}
		}
	}
	return opts, true
}

var protectedNetworks = map[string]struct{}{
	"bridge": {}, "host": {}, "none": {},
	"dockfin": {}, "coolify": {},
}

func dockerMissingOK(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "no such") ||
		strings.Contains(s, "not found") ||
		strings.Contains(s, "does not exist") ||
		strings.Contains(s, "no such file") ||
		strings.Contains(s, "no such container") ||
		strings.Contains(s, "no such volume") ||
		strings.Contains(s, "no such network")
}

func runArgsRetry(client *ssh.Client, tries int, args ...string) {
	if tries < 1 {
		tries = 1
	}
	for i := 0; i < tries; i++ {
		_, errOut, err := sshx.RunArgs(client, args...)
		if err == nil || dockerMissingOK(errOut) {
			return
		}
	}
}

// removeResourceScopedNetwork mirrors Coolify deleteConnectedNetworks: remove a network
// named after the resource UUID only. Never touches shared destination/proxy networks.
func removeResourceScopedNetwork(client *ssh.Client, resourceID string) {
	name := strings.TrimSpace(resourceID)
	if name == "" {
		return
	}
	if _, protected := protectedNetworks[name]; protected {
		return
	}
	runArgsRetry(client, 2, "docker", "network", "disconnect", name, "dockfin-proxy")
	runArgsRetry(client, 2, "docker", "network", "rm", name)
}

// runDockerCleanup approximates Coolify CleanupDocker for delete flows.
func runDockerCleanup(client *ssh.Client) {
	runArgsRetry(client, 2, "docker", "image", "prune", "-f")
	runArgsRetry(client, 2, "docker", "builder", "prune", "-f")
}

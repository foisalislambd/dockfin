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
	_, _, _ = sshx.RunArgs(client, "docker", "network", "disconnect", name, "dockfin-proxy")
	_, _, _ = sshx.RunArgs(client, "docker", "network", "rm", name)
}

// runDockerCleanup approximates Coolify CleanupDocker for delete flows.
func runDockerCleanup(client *ssh.Client) {
	_, _, _ = sshx.RunArgs(client, "docker", "image", "prune", "-f")
	_, _, _ = sshx.RunArgs(client, "docker", "builder", "prune", "-f")
}

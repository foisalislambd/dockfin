package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

const cloudflaredContainer = "dockfin-cloudflared"

// handleCloudflareTunnelAction installs, stops, or reports the cloudflared
// connector container on the server (Coolify-style `tunnel run --token`).
func (a *API) handleCloudflareTunnelAction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	action := chi.URLParam(r, "action")
	teamID := currentTeamID(r)
	ops, err := a.Store.GetServerOpsSettings(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	// Allow passing the token inline so the first install does not need a
	// separate ops PATCH round-trip.
	var body struct {
		Token string `json:"cloudflare_tunnel_token"`
	}
	if err := decodeJSONOptional(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if tok := strings.TrimSpace(body.Token); tok != "" && !strings.Contains(tok, "…") {
		ops.CloudflareTunnelToken = tok
	}

	client, err := a.dialServer(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch action {
	case "install", "restart":
		if strings.TrimSpace(ops.CloudflareTunnelToken) == "" {
			writeError(w, http.StatusBadRequest, "cloudflare tunnel token is required")
			return
		}
		ops.CloudflareTunnelEnabled = true
		if err := a.Store.SetServerOpsSettings(r.Context(), teamID, id, *ops); err != nil {
			mapStoreErr(w, err)
			return
		}
		if err := installCloudflared(client, ops.CloudflareTunnelToken); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": cloudflaredStatus(client)})
	case "stop":
		_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", cloudflaredContainer)
		ops.CloudflareTunnelEnabled = false
		_ = a.Store.SetServerOpsSettings(r.Context(), teamID, id, *ops)
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
	case "status":
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  cloudflaredStatus(client),
			"enabled": ops.CloudflareTunnelEnabled,
		})
	default:
		writeError(w, http.StatusBadRequest, "action must be install, restart, stop, or status")
	}
}

func installCloudflared(client *ssh.Client, token string) error {
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", cloudflaredContainer)
	_, errOut, err := sshx.RunArgs(client, "docker", "run", "-d",
		"--name", cloudflaredContainer,
		"--restart", "unless-stopped",
		"--network", "host",
		"cloudflare/cloudflared:latest",
		"tunnel", "--no-autoupdate", "run", "--token", token,
	)
	if err != nil {
		return fmt.Errorf("start cloudflared: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}

func cloudflaredStatus(client *ssh.Client) string {
	out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Status}}", cloudflaredContainer)
	if err != nil {
		return "not installed"
	}
	if s := strings.TrimSpace(out); s != "" {
		return s
	}
	return "unknown"
}

// handleCheckServerPatches reports pending OS package updates so admins can see
// outstanding security patches without opening a terminal.
func (a *API) handleCheckServerPatches(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	client, err := a.dialServer(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// `yum check-update` / `apk` exit non-zero when updates exist, so ignore the
	// exit status and report whatever the package manager printed.
	out, errOut, _ := sshx.Run(client, `if command -v apt-get >/dev/null 2>&1; then `+
		`apt-get update -qq >/dev/null 2>&1; apt list --upgradable 2>/dev/null | tail -n +2; `+
		`elif command -v dnf >/dev/null 2>&1; then dnf -q check-update; `+
		`elif command -v yum >/dev/null 2>&1; then yum -q check-update; `+
		`elif command -v apk >/dev/null 2>&1; then apk update >/dev/null 2>&1; apk list --upgradable; `+
		`else echo "no supported package manager found"; fi`)
	output := strings.TrimSpace(out)
	if output == "" {
		output = strings.TrimSpace(errOut)
	}
	count := 0
	if output != "" && !strings.HasPrefix(output, "no supported package manager") {
		for _, line := range strings.Split(output, "\n") {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
	}
	if output == "" {
		output = "No pending updates."
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": output, "count": count})
}

// applyEdgeSettings pushes log drain config and the custom CA certificate to the
// server. Failures are returned as warnings so saving settings never blocks.
func applyEdgeSettings(client *ssh.Client, ops store.ServerOpsSettings, syncDrain, syncCA bool) []string {
	var warnings []string
	_ = sshx.EnsureDataDirs(client)
	if syncDrain {
		if err := syncLogDrain(client, ops); err != nil {
			warnings = append(warnings, "log drain: "+err.Error())
		}
	}
	if syncCA {
		if strings.TrimSpace(ops.CACertificate) != "" {
			_, _, _ = sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/ca")
			if err := sshx.WriteFile(client, "/data/dockfin/ca/custom-ca.crt", []byte(ops.CACertificate)); err != nil {
				warnings = append(warnings, "ca certificate: "+err.Error())
			}
		} else {
			_, _, _ = sshx.RunArgs(client, "rm", "-f", "/data/dockfin/ca/custom-ca.crt")
		}
	}
	return warnings
}

const logDrainContainer = "dockfin-log-drain"

func syncLogDrain(client *ssh.Client, ops store.ServerOpsSettings) error {
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", logDrainContainer)
	if !ops.LogDrainEnabled {
		_, _, _ = sshx.RunArgs(client, "rm", "-f", "/data/dockfin/log-drain.env", "/data/dockfin/log-drain/vector.yaml")
		return nil
	}
	cfg, err := vectorConfig(ops.LogDrainType, ops.LogDrainConfig)
	if err != nil {
		return err
	}
	env := fmt.Sprintf("LOG_DRAIN_ENABLED=true\nLOG_DRAIN_TYPE=%s\nLOG_DRAIN_CONFIG=%s\n",
		ops.LogDrainType, strings.ReplaceAll(ops.LogDrainConfig, "\n", " "))
	if err := sshx.WriteFile(client, "/data/dockfin/log-drain.env", []byte(env)); err != nil {
		return err
	}
	_, _, _ = sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/log-drain")
	if err := sshx.WriteFile(client, "/data/dockfin/log-drain/vector.yaml", []byte(cfg)); err != nil {
		return err
	}
	_, errOut, err := sshx.RunArgs(client, "docker", "run", "-d",
		"--name", logDrainContainer,
		"--restart", "unless-stopped",
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		"-v", "/data/dockfin/log-drain/vector.yaml:/etc/vector/vector.yaml:ro",
		"timberio/vector:0.41.1-alpine",
		"--config", "/etc/vector/vector.yaml",
	)
	if err != nil {
		return fmt.Errorf("start log drain: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}

func vectorConfig(kind, raw string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	raw = strings.TrimSpace(raw)
	var cfg map[string]any
	_ = json.Unmarshal([]byte(raw), &cfg)
	uri := raw
	headers := ""
	if cfg != nil {
		if v, _ := cfg["endpoint"].(string); v != "" {
			uri = v
		}
		if v, _ := cfg["url"].(string); v != "" {
			uri = v
		}
		switch kind {
		case "newrelic":
			if v, _ := cfg["api_key"].(string); v != "" {
				headers = "      Api-Key: \"" + strings.ReplaceAll(v, `"`, ``) + "\"\n"
			}
			if uri == raw || uri == "" {
				uri = "https://log-api.newrelic.com/log/v1"
			}
		case "axiom":
			dataset, _ := cfg["dataset"].(string)
			token, _ := cfg["token"].(string)
			if dataset == "" {
				dataset = "dockfin"
			}
			uri = "https://api.axiom.co/v1/datasets/" + dataset + "/ingest"
			if token != "" {
				headers = "      Authorization: \"Bearer " + strings.ReplaceAll(token, `"`, ``) + "\"\n"
			}
		}
	}
	if uri == "" {
		return "", fmt.Errorf("log drain config needs an endpoint/url")
	}
	if headers == "" {
		return fmt.Sprintf(`sources:
  docker_logs:
    type: docker_logs
    exclude_containers:
      - %s
sinks:
  out:
    type: http
    inputs: ["docker_logs"]
    uri: %q
    encoding:
      codec: json
`, logDrainContainer, uri), nil
	}
	return fmt.Sprintf(`sources:
  docker_logs:
    type: docker_logs
    exclude_containers:
      - %s
sinks:
  out:
    type: http
    inputs: ["docker_logs"]
    uri: %q
    encoding:
      codec: json
    headers:
%s`, logDrainContainer, uri, headers), nil
}

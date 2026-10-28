package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleGetServerOps(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ops, err := a.Store.GetServerOpsSettings(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	// Never echo full token unless requested; mask mid for UI.
	out := *ops
	if out.SentinelToken != "" {
		out.SentinelToken = maskToken(out.SentinelToken)
	}
	if out.CloudflareTunnelToken != "" {
		out.CloudflareTunnelToken = maskToken(out.CloudflareTunnelToken)
	}
	writeJSON(w, http.StatusOK, out)
}

func maskToken(t string) string {
	if len(t) <= 8 {
		return "••••••••"
	}
	return t[:4] + "…" + t[len(t)-4:]
}

func (a *API) handlePatchServerOps(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	cur, err := a.Store.GetServerOpsSettings(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var body struct {
		SentinelEnabled                   *bool     `json:"sentinel_enabled"`
		SentinelToken                     *string   `json:"sentinel_token"`
		SentinelMetricsRefreshRateSeconds *int      `json:"sentinel_metrics_refresh_rate_seconds"`
		DockerCleanupFrequency            *string   `json:"docker_cleanup_frequency"`
		DockerCleanupThreshold            *int      `json:"docker_cleanup_threshold"`
		ForceDockerCleanup                *bool     `json:"force_docker_cleanup"`
		CloudflareTunnelToken             *string   `json:"cloudflare_tunnel_token"`
		CloudflareTunnelEnabled           *bool     `json:"cloudflare_tunnel_enabled"`
		LogDrainEnabled                   *bool     `json:"log_drain_enabled"`
		LogDrainType                      *string   `json:"log_drain_type"`
		LogDrainConfig                    *string   `json:"log_drain_config"`
		CACertificate                     *string   `json:"ca_certificate"`
		TerminalACLUserIDs                *[]string `json:"terminal_acl_user_ids"`
		TrafficAnalyticsEnabled           *bool     `json:"traffic_analytics_enabled"`
		TrafficSampleThreshold            *int      `json:"traffic_sample_threshold"`
		TrafficRetentionDays              *int      `json:"traffic_retention_days"`
		TrafficIPMode                     *string   `json:"traffic_ip_mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	next := *cur
	if body.SentinelEnabled != nil {
		next.SentinelEnabled = *body.SentinelEnabled
	}
	if body.SentinelToken != nil {
		tok := strings.TrimSpace(*body.SentinelToken)
		if tok != "" && !strings.Contains(tok, "…") && !strings.HasPrefix(tok, "••") {
			next.SentinelToken = tok
		}
	}
	if body.SentinelMetricsRefreshRateSeconds != nil {
		next.SentinelMetricsRefreshRateSeconds = *body.SentinelMetricsRefreshRateSeconds
	}
	if body.DockerCleanupFrequency != nil {
		next.DockerCleanupFrequency = strings.TrimSpace(*body.DockerCleanupFrequency)
	}
	if body.DockerCleanupThreshold != nil {
		next.DockerCleanupThreshold = *body.DockerCleanupThreshold
	}
	if body.ForceDockerCleanup != nil {
		next.ForceDockerCleanup = *body.ForceDockerCleanup
	}
	if body.CloudflareTunnelToken != nil {
		tok := strings.TrimSpace(*body.CloudflareTunnelToken)
		if tok != "" && !strings.Contains(tok, "…") && !strings.HasPrefix(tok, "••") {
			next.CloudflareTunnelToken = tok
		} else if tok == "" {
			next.CloudflareTunnelToken = ""
		}
	}
	if body.CloudflareTunnelEnabled != nil {
		next.CloudflareTunnelEnabled = *body.CloudflareTunnelEnabled
	}
	if body.LogDrainEnabled != nil {
		next.LogDrainEnabled = *body.LogDrainEnabled
	}
	if body.LogDrainType != nil {
		next.LogDrainType = strings.TrimSpace(*body.LogDrainType)
	}
	if body.LogDrainConfig != nil {
		next.LogDrainConfig = *body.LogDrainConfig
	}
	if body.CACertificate != nil {
		next.CACertificate = *body.CACertificate
	}
	if body.TerminalACLUserIDs != nil {
		next.TerminalACLUserIDs = *body.TerminalACLUserIDs
	}
	if body.TrafficSampleThreshold != nil {
		if *body.TrafficSampleThreshold < 0 || *body.TrafficSampleThreshold > 100000 {
			writeError(w, http.StatusBadRequest, "sample threshold must be from 0 to 100000")
			return
		}
		next.TrafficSampleThreshold = *body.TrafficSampleThreshold
	}
	if body.TrafficRetentionDays != nil {
		if *body.TrafficRetentionDays < 1 || *body.TrafficRetentionDays > 90 {
			writeError(w, http.StatusBadRequest, "retention must be from 1 to 90 days")
			return
		}
		next.TrafficRetentionDays = *body.TrafficRetentionDays
	}
	if body.TrafficIPMode != nil {
		mode := strings.TrimSpace(*body.TrafficIPMode)
		if mode != "full" && mode != "anonymized" && mode != "off" {
			writeError(w, http.StatusBadRequest, "ip mode must be full, anonymized, or off")
			return
		}
		next.TrafficIPMode = mode
	}
	if body.TrafficAnalyticsEnabled != nil && *body.TrafficAnalyticsEnabled {
		srv, err := a.Store.GetServer(r.Context(), teamID, id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if srv.ProxyType == "caddy" || srv.ProxyType == "none" {
			writeError(w, http.StatusBadRequest, "traffic analytics needs the Traefik proxy")
			return
		}
		next.TrafficAnalyticsEnabled = true
	} else if body.TrafficAnalyticsEnabled != nil {
		next.TrafficAnalyticsEnabled = false
	}
	if err := a.Store.SetServerOpsSettings(r.Context(), teamID, id, next); err != nil {
		mapStoreErr(w, err)
		return
	}
	if body.TrafficIPMode != nil && next.TrafficIPMode != cur.TrafficIPMode {
		_ = a.Store.ClearTrafficClients(r.Context(), id)
	}
	syncDrain := body.LogDrainEnabled != nil || body.LogDrainType != nil || body.LogDrainConfig != nil
	syncCA := body.CACertificate != nil
	syncTraffic := cur.TrafficAnalyticsEnabled != next.TrafficAnalyticsEnabled
	out := map[string]any{"status": "updated"}
	var warnings []string
	if syncDrain || syncCA || syncTraffic {
		client, dialErr := a.dialServer(r, id)
		if dialErr != nil {
			warnings = append(warnings, "settings saved but not applied on the server: "+dialErr.Error())
		} else {
			if syncDrain || syncCA {
				warnings = append(warnings, applyEdgeSettings(client, next, syncDrain, syncCA)...)
			}
			if syncTraffic {
				warnings = append(warnings, a.applyTrafficAnalytics(r, client, id, next)...)
			}
		}
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleRotateSentinelToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	ops, err := a.Store.GetServerOpsSettings(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	tok, err := crypto.RandomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	ops.SentinelToken = tok
	if err := a.Store.SetServerOpsSettings(r.Context(), teamID, id, *ops); err != nil {
		mapStoreErr(w, err)
		return
	}
	// Agent embeds the token in run.sh — reinstall when enabled so ingest keeps working.
	if ops.SentinelEnabled {
		if client, dialErr := a.dialServer(r, id); dialErr == nil {
			if err := installSentinel(client, a.Cfg.PublicURL, id.String(), tok, ops.SentinelMetricsRefreshRateSeconds); err != nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"sentinel_token": tok,
					"warning":        "token saved but sentinel reinstall failed: " + err.Error(),
				})
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"sentinel_token": tok})
}

func (a *API) handleSentinelAction(w http.ResponseWriter, r *http.Request) {
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
	client, err := a.dialServer(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch action {
	case "install", "restart":
		if ops.SentinelToken == "" {
			tok, err := crypto.RandomToken(32)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "token generation failed")
				return
			}
			ops.SentinelToken = tok
		}
		ops.SentinelEnabled = true
		if err := a.Store.SetServerOpsSettings(r.Context(), teamID, id, *ops); err != nil {
			mapStoreErr(w, err)
			return
		}
		if err := installSentinel(client, a.Cfg.PublicURL, id.String(), ops.SentinelToken, ops.SentinelMetricsRefreshRateSeconds); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "running", "sentinel_token": ops.SentinelToken})
	case "stop":
		_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", "dockfin-sentinel")
		ops.SentinelEnabled = false
		_ = a.Store.SetServerOpsSettings(r.Context(), teamID, id, *ops)
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
	default:
		writeError(w, http.StatusBadRequest, "action must be install, restart, or stop")
	}
}

func (a *API) applyTrafficAnalytics(r *http.Request, client *ssh.Client, serverID uuid.UUID, ops store.ServerOpsSettings) []string {
	var warnings []string
	enabled := "0"
	if ops.TrafficAnalyticsEnabled {
		enabled = "1"
	}
	body := fmt.Sprintf("ENABLED=%s\nMODE=%s\nTHRESHOLD=%d\n", enabled, ops.TrafficIPMode, ops.TrafficSampleThreshold)
	_, _, _ = sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/sentinel")
	if err := sshx.WriteFile(client, "/data/dockfin/sentinel/traffic.env", []byte(body)); err != nil {
		return []string{"traffic settings saved but the server file was not written: " + err.Error()}
	}
	if proxy.ProxyStatus(client) == "running" {
		srv, err := a.Store.GetServer(r.Context(), currentTeamID(r), serverID)
		if err != nil {
			warnings = append(warnings, "traffic file written but the proxy was not restarted: "+err.Error())
		} else if srv.ProxyType == "" || srv.ProxyType == "traefik" {
			network := "dockfin"
			if dests, err := a.Store.ListDestinations(r.Context(), currentTeamID(r), &serverID); err == nil && len(dests) > 0 && dests[0].Network != "" {
				network = dests[0].Network
			}
			if err := proxy.StartProxy(client, srv.ProxyType, a.Cfg.TraefikImage, a.Cfg.CaddyImage, network, a.Cfg.AcmeEmail); err != nil {
				warnings = append(warnings, "traffic file written but the proxy was not restarted: "+err.Error())
			} else {
				_ = proxy.EnsureProxyJoinedResourceNetworks(client)
				_ = a.Store.UpdateServerProxyStatus(r.Context(), serverID, "running")
			}
		}
	}
	if ops.SentinelEnabled {
		if err := installSentinel(client, a.Cfg.PublicURL, serverID.String(), ops.SentinelToken, ops.SentinelMetricsRefreshRateSeconds); err != nil {
			warnings = append(warnings, "proxy updated but Sentinel was not restarted: "+err.Error())
		}
	} else if ops.TrafficAnalyticsEnabled {
		warnings = append(warnings, "request collection is on, but Sentinel is not running, so no lines are sent")
	}
	return warnings
}

func installSentinel(client *ssh.Client, publicURL, serverID, token string, refreshSec int) error {
	if refreshSec <= 0 {
		refreshSec = 30
	}
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if publicURL == "" {
		return fmt.Errorf("DOCKFIN_PUBLIC_URL is required to install sentinel")
	}
	_ = sshx.EnsureDataDirs(client)
	_, _, _ = sshx.Run(client, "mkdir -p /data/dockfin/sentinel")
	script := fmt.Sprintf(`#!/bin/sh
set -eu
URL=%q
TOKEN=%q
SERVER_ID=%q
INTERVAL=%d
PROC=/host/proc
# Sample /proc/stat twice so cpu_percent is a short-interval utilization, not
# the lifetime average since boot.
read_cpu() {
  awk '/^cpu / {print $2+$4, $2+$4+$5}' "$PROC/stat"
}
push_traffic() {
  ENV=/hostroot/data/dockfin/sentinel/traffic.env
  LOG=/hostroot/data/dockfin/proxy/traefik/access.log
  [ -f "$ENV" ] || return 0
  [ -f "$LOG" ] || return 0
  enabled=$(sed -n 's/^ENABLED=//p' "$ENV" | head -n 1)
  [ "$enabled" = "1" ] || return 0
  size=$(wc -c < "$LOG" | tr -d ' ')
  if [ ! -f /tmp/access.offset ]; then
    echo "$size" > /tmp/access.offset
    return 0
  fi
  off=$(tr -d ' \n' < /tmp/access.offset)
  case $off in ''|*[!0-9]*) off=0 ;; esac
  if [ "$size" -lt "$off" ]; then off=0; fi
  if [ "$size" -eq "$off" ]; then return 0; fi
  dd if="$LOG" bs=1 skip="$off" count=262144 of=/tmp/access.chunk 2>/dev/null || return 0
  # Count original bytes through the last complete line. A partial last line stays
  # for the next read. Advancing past it made the offset larger than the file,
  # and the next loop then read the file again from the start.
  ends=$(tail -c 1 /tmp/access.chunk | od -An -tu1 | tr -d ' \n')
  awk -v ends="$ends" '
    { n = NR; raw[NR] = $0; blen[NR] = length($0) + 1 }
    END {
      last = n
      if (last > 400) last = 400
      if (n <= 400 && ends != "10" && last > 0) last = last - 1
      total = 0
      for (i = 1; i <= last; i++) total += blen[i]
      print total
      for (i = 1; i <= last; i++) print raw[i]
    }
  ' /tmp/access.chunk > /tmp/access.meta || return 0
  used=$(head -n 1 /tmp/access.meta | tr -d ' \n')
  case $used in ''|*[!0-9]*) return 0 ;; esac
  chunk=$(wc -c < /tmp/access.chunk | tr -d ' ')
  if [ "$used" -eq 0 ] || [ "$used" -gt "$chunk" ]; then return 0; fi
  tail -n +2 /tmp/access.meta > /tmp/access.lines
  [ -s /tmp/access.lines ] || return 0
  jq -R -s --arg sid "$SERVER_ID" --arg tok "$TOKEN" '{server_id:$sid,token:$tok,lines:(split("\n")|map(select(length>0)))}' /tmp/access.lines > /tmp/access.json || return 0
  if wget -q -O /dev/null --header="Content-Type: application/json" --post-file=/tmp/access.json "$URL/api/v1/sentinel/traffic"; then
    echo $((off + used)) > /tmp/access.offset
  fi
  rm -f /tmp/access.json /tmp/access.lines /tmp/access.chunk /tmp/access.meta
}
while true; do
  set -- $(read_cpu)
  U1=$1 T1=$2
  sleep 1
  set -- $(read_cpu)
  U2=$1 T2=$2
  DU=$((U2-U1)); DT=$((T2-T1))
  if [ "$DT" -gt 0 ]; then CPU=$(awk -v du="$DU" -v dt="$DT" 'BEGIN { printf "%%.2f", 100*du/dt }'); else CPU=0; fi
  MEM_TOTAL=$(awk '/MemTotal/ {print $2*1024}' "$PROC/meminfo")
  MEM_AVAIL=$(awk '/MemAvailable/ {print $2*1024}' "$PROC/meminfo")
  MEM_USED=$((MEM_TOTAL-MEM_AVAIL))
  DISK_TOTAL=$(df -B1 /hostroot 2>/dev/null | awk 'NR==2 {print $2}')
  DISK_USED=$(df -B1 /hostroot 2>/dev/null | awk 'NR==2 {print $3}')
  : "${DISK_TOTAL:=0}"
  : "${DISK_USED:=0}"
  BODY=$(printf '{"server_id":"%%s","token":"%%s","cpu_percent":%%s,"memory_used_bytes":%%s,"memory_total_bytes":%%s,"disk_used_bytes":%%s,"disk_total_bytes":%%s}' \
    "$SERVER_ID" "$TOKEN" "$CPU" "$MEM_USED" "$MEM_TOTAL" "$DISK_USED" "$DISK_TOTAL")
  wget -q -O /dev/null --header="Content-Type: application/json" --post-data="$BODY" "$URL/api/v1/sentinel/metrics" 2>/dev/null || true
  push_traffic || true
  # INTERVAL already includes the 1s sample sleep above.
  REST=$((INTERVAL-1))
  if [ "$REST" -gt 0 ]; then sleep "$REST"; fi
done
`, publicURL, token, serverID, refreshSec)
	if err := sshx.WriteFile(client, "/data/dockfin/sentinel/run.sh", []byte(script)); err != nil {
		return err
	}
	_, _, _ = sshx.RunArgs(client, "chmod", "+x", "/data/dockfin/sentinel/run.sh")
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", "dockfin-sentinel")
	_, errOut, err := sshx.RunArgs(client, "docker", "run", "-d",
		"--name", "dockfin-sentinel",
		"--restart", "unless-stopped",
		"--network", "host",
		"-v", "/data/dockfin/sentinel/run.sh:/run.sh:ro",
		"-v", "/proc:/host/proc:ro",
		"-v", "/:/hostroot:ro",
		"alpine:3.21",
		"sh", "-c", "apk add --no-cache wget jq >/dev/null && exec sh /run.sh",
	)
	if err != nil {
		return fmt.Errorf("start sentinel: %v %s", err, errOut)
	}
	return nil
}

func (a *API) handleSentinelLogs(w http.ResponseWriter, r *http.Request) {
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
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}
	out, errOut, err := sshx.RunArgs(client, "docker", "logs", "--tail", tail, "dockfin-sentinel")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"logs": strings.TrimSpace(errOut + "\n" + out), "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": out})
}

func (a *API) handleRunDockerCleanup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	ops, err := a.Store.GetServerOpsSettings(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	exec, err := a.Store.CreateDockerCleanupExecution(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, id)
	if err != nil {
		_ = a.Store.FinishDockerCleanupExecution(r.Context(), exec.ID, "failed", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg, runErr := runDockerCleanupDetailed(client, ops.ForceDockerCleanup, ops.DockerCleanupThreshold)
	status := "finished"
	if runErr != nil {
		status = "failed"
		if msg == "" {
			msg = runErr.Error()
		} else {
			msg = msg + "; " + runErr.Error()
		}
	}
	_ = a.Store.FinishDockerCleanupExecution(r.Context(), exec.ID, status, msg)
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "message": msg, "execution_id": exec.ID})
}

func (a *API) handleListDockerCleanupExecutions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.Store.ListDockerCleanupExecutions(r.Context(), currentTeamID(r), id, limit)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.DockerCleanupExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": list})
}

// handleListDockerCleanupExecutionsForTeam returns recent docker cleanup executions across all
// servers for the current team, optionally filtered by ?status=failed.
func (a *API) handleListDockerCleanupExecutionsForTeam(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.Store.ListDockerCleanupExecutionsForTeam(r.Context(), currentTeamID(r), status, limit)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": list})
}

func runDockerCleanupDetailed(client *ssh.Client, force bool, threshold int) (string, error) {
	var parts []string
	if !force && threshold > 0 {
		out, _, err := sshx.Run(client, `df -P / | awk 'NR==2 {gsub(/%/,"",$5); print $5}'`)
		if err == nil {
			used, _ := strconv.Atoi(strings.TrimSpace(out))
			if used > 0 && used < threshold {
				return fmt.Sprintf("skipped: disk usage %d%% below threshold %d%%", used, threshold), nil
			}
			parts = append(parts, fmt.Sprintf("disk usage %d%%", used))
		}
	}
	_, errOut1, err1 := sshx.RunArgs(client, "docker", "image", "prune", "-af")
	if err1 != nil {
		return strings.Join(parts, "; "), fmt.Errorf("image prune: %v %s", err1, errOut1)
	}
	parts = append(parts, "image prune ok")
	_, errOut2, err2 := sshx.RunArgs(client, "docker", "builder", "prune", "-af")
	if err2 != nil {
		return strings.Join(parts, "; "), fmt.Errorf("builder prune: %v %s", err2, errOut2)
	}
	parts = append(parts, "builder prune ok")
	_, _, _ = sshx.RunArgs(client, "docker", "container", "prune", "-f")
	parts = append(parts, "container prune ok")
	return strings.Join(parts, "; "), nil
}

func (a *API) handleListProxyDynamic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	list, err := a.Store.ListServerProxyConfigurations(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.ServerProxyConfiguration{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"configurations": list})
}

func (a *API) handleUpsertProxyDynamic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	name := strings.TrimSpace(body.Name)
	if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
		name += ".yml"
	}
	cfg, err := a.Store.UpsertServerProxyConfiguration(r.Context(), currentTeamID(r), id, name, body.Value)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, dialErr := a.dialServer(r, id)
	if dialErr == nil {
		_ = sshx.EnsureDataDirs(client)
		_, _, _ = sshx.Run(client, "mkdir -p /data/dockfin/proxy/traefik/dynamic")
		safeName := strings.ReplaceAll(name, "/", "_")
		_ = sshx.WriteFile(client, "/data/dockfin/proxy/traefik/dynamic/"+safeName, []byte(body.Value))
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (a *API) handleDeleteProxyDynamic(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	cfgID, err := uuid.Parse(chi.URLParam(r, "configID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	teamID := currentTeamID(r)
	list, _ := a.Store.ListServerProxyConfigurations(r.Context(), teamID, serverID)
	var name string
	for _, c := range list {
		if c.ID == cfgID {
			name = c.Name
			break
		}
	}
	if err := a.Store.DeleteServerProxyConfiguration(r.Context(), teamID, serverID, cfgID); err != nil {
		mapStoreErr(w, err)
		return
	}
	if name != "" {
		if client, err := a.dialServer(r, serverID); err == nil {
			safeName := strings.ReplaceAll(name, "/", "_")
			_, _, _ = sshx.RunArgs(client, "rm", "-f", "/data/dockfin/proxy/traefik/dynamic/"+safeName)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleProxyLogs(w http.ResponseWriter, r *http.Request) {
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
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}
	out, errOut, err := sshx.RunArgs(client, "docker", "logs", "--tail", tail, "dockfin-proxy")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"logs": strings.TrimSpace(errOut + "\n" + out), "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": out})
}

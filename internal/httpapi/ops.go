package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/terminal"
	"github.com/dockfin/dockfin/internal/worker"
)

func (a *API) handleListServices(w http.ResponseWriter, r *http.Request) {
	var envID *uuid.UUID
	if s := r.URL.Query().Get("environment_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		envID = &id
	}
	list, err := a.Store.ListServices(r.Context(), currentTeamID(r), envID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": list})
}

func (a *API) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnvironmentID    string `json:"environment_id"`
		ServerID         string `json:"server_id"`
		DestinationID    string `json:"destination_id"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		ServiceType      string `json:"service_type"`
		DockerComposeRaw string `json:"docker_compose_raw"`
		Template         string `json:"template"`
		FQDN             string `json:"fqdn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	envID, err := uuid.Parse(body.EnvironmentID)
	if err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "environment_id and name required")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetEnvironment(r.Context(), teamID, envID); err != nil {
		mapStoreErr(w, err)
		return
	}
	compose := body.DockerComposeRaw
	svcType := body.ServiceType
	if body.Template != "" {
		tpl, err := services.GetTemplate(body.Template)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		compose = tpl.Compose
		svcType = tpl.Type
	}
	if svcType == "" {
		svcType = "custom"
	}
	svc := &store.Service{
		TeamID: teamID, EnvironmentID: envID, Name: body.Name, Description: body.Description,
		ServiceType: svcType, DockerComposeRaw: compose, FQDN: strings.TrimSpace(body.FQDN),
	}
	if body.ServerID != "" {
		id, err := uuid.Parse(body.ServerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid server_id")
			return
		}
		if _, err := a.Store.GetServer(r.Context(), teamID, id); err != nil {
			mapStoreErr(w, err)
			return
		}
		svc.ServerID = &id
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		if _, err := a.Store.GetDestination(r.Context(), teamID, id); err != nil {
			mapStoreErr(w, err)
			return
		}
		svc.DestinationID = &id
	}

	if svc.FQDN != "" && !isValidHostnameList(svc.FQDN) {
		writeError(w, http.StatusBadRequest, "invalid fqdn — use hostnames like example.com or https://example.com (comma-separated OK)")
		return
	}
	svc.FQDN = proxy.NormalizeDomains(svc.FQDN)

	opts := services.PrepareOpts{BaseURL: "http://127.0.0.1"}
	if svc.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), teamID, *svc.DestinationID); err == nil {
			opts.Network = dest.Network
		}
	}
	if srv, err := a.resolveServerForDomain(r.Context(), teamID, svc.ServerID, svc.DestinationID); err == nil {
		// Pre-assign ID so FQDN short-id is stable before insert.
		svc.ID = uuid.New()
		if svc.FQDN == "" {
			svc.FQDN = generateResourceFQDN(svc.Name, svc.ID, srv)
			svc.FQDN = proxy.NormalizeDomains(svc.FQDN)
		}
		if svc.FQDN != "" {
			opts.BaseURL = proxy.AutoPublicURL(svc.FQDN)
			opts.FQDN = svc.FQDN
			// Unique Traefik router/service names — plain svc.Name collides when
			// two resources share a name (e.g. two Planka installs → 404).
			opts.RouterName = svc.Name + "-" + svc.ID.String()[:8]
			opts.ServiceID = svc.ID.String()
		}
	} else if svc.FQDN != "" {
		// Domain set without resolvable server — still bake Traefik labels / URL env.
		opts.BaseURL = proxy.AutoPublicURL(svc.FQDN)
		opts.FQDN = svc.FQDN
	}
	prepared, fullEnv, err := services.PrepareCompose(compose, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose: %v", err))
		return
	}
	svc.DockerCompose = prepared

	created, err := a.Store.CreateService(r.Context(), svc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	a.syncServiceCoolifyEnv(r.Context(), teamID, created.ID, compose, prepared, fullEnv)
	a.syncResourceComposeEnvRefs(r.Context(), teamID, "service", created.ID, compose)
	writeJSON(w, http.StatusCreated, serviceWithLinks(created))
}

func (a *API) handleGetService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	svc, err := a.Store.GetService(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, serviceWithLinks(svc))
}

func serviceWithLinks(svc *store.Service) map[string]any {
	compose := svc.DockerCompose
	if compose == "" {
		compose = svc.DockerComposeRaw
	}
	links := proxy.CollectLinks(svc.FQDN, compose)
	if len(links) == 0 && svc.FQDN != "" {
		links = proxy.CollectLinks(svc.FQDN, "")
	}
	units := services.ParseComposeUnits(compose)
	assigned := map[string]bool{}
	unitRows := make([]map[string]any, 0, len(units))
	for _, u := range units {
		unitLinks := matchUnitLinks(u.Name, links)
		for _, l := range unitLinks {
			assigned[l.URL] = true
		}
		unitRows = append(unitRows, map[string]any{
			"name":   u.Name,
			"image":  u.Image,
			"links":  unitLinks,
			"status": svc.Status,
		})
	}
	if len(unitRows) > 0 {
		var leftover []proxy.ResourceLink
		for _, l := range links {
			if !assigned[l.URL] {
				leftover = append(leftover, l)
			}
		}
		if len(leftover) > 0 {
			existing, _ := unitRows[0]["links"].([]proxy.ResourceLink)
			unitRows[0]["links"] = append(existing, leftover...)
		}
	}
	volumes := services.ParseComposeVolumes(compose)
	return map[string]any{
		"id":                 svc.ID,
		"team_id":            svc.TeamID,
		"environment_id":     svc.EnvironmentID,
		"server_id":          svc.ServerID,
		"destination_id":     svc.DestinationID,
		"name":               svc.Name,
		"description":        svc.Description,
		"service_type":       svc.ServiceType,
		"docker_compose_raw": svc.DockerComposeRaw,
		"docker_compose":     svc.DockerCompose,
		"fqdn":               svc.FQDN,
		"status":             svc.Status,
		"created_at":         svc.CreatedAt,
		"links":              links,
		"units":              unitRows,
		"volumes":            volumes,
	}
}

func matchUnitLinks(unitName string, links []proxy.ResourceLink) []proxy.ResourceLink {
	needle := strings.ToUpper(strings.ReplaceAll(unitName, "-", "_"))
	var out []proxy.ResourceLink
	for _, l := range links {
		label := strings.ToUpper(strings.ReplaceAll(l.Label, " ", "_"))
		if label == "WEB" {
			continue
		}
		if label == needle || strings.HasPrefix(label, needle+"_") || strings.HasPrefix(needle, label+"_") {
			out = append(out, l)
		}
	}
	return out
}

func (a *API) handleDeployService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if svc.DockerComposeRaw == "" && svc.DockerCompose == "" {
		writeError(w, http.StatusBadRequest, "service has no docker compose content")
		return
	}
	serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stream := r.URL.Query().Get("stream") == "1" ||
		strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"
	isWebhook := r.URL.Query().Get("uuid") != "" || strings.Contains(r.URL.Path, "/webhooks/")

	sid := serverID
	dep, err := a.Store.CreateServiceDeployment(r.Context(), teamID, id, &sid, force, isWebhook, !isWebhook)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if a.Queue == nil {
		writeError(w, http.StatusServiceUnavailable, "deploy queue unavailable")
		return
	}

	if stream {
		// Subscribe / open SSE before enqueue so early worker logs are not missed.
		a.streamQueuedDeployment(w, r, teamID, dep.ID, func() error {
			return a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: teamID, ForceRebuild: force})
		})
		return
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: teamID, ForceRebuild: force}); err != nil {
		writeError(w, http.StatusInternalServerError, "enqueue failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "queued",
		"deployment_id": dep.ID,
		"resource_type": "service",
		"uuid":          id,
	})
}

// streamQueuedDeployment bridges Hub publish events to an SSE response until the job finishes.
// optionalEnqueue runs after the SSE subscription is ready (avoids missing early log lines).
func (a *API) streamQueuedDeployment(w http.ResponseWriter, r *http.Request, teamID, depID uuid.UUID, optionalEnqueue func() error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	emit := func(payload map[string]string) {
		b, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	var ch chan []byte
	if a.Hub != nil {
		ch = a.Hub.Subscribe(depID)
		defer a.Hub.Unsubscribe(depID, ch)
	}

	if optionalEnqueue != nil {
		if err := optionalEnqueue(); err != nil {
			emit(map[string]string{"stage": "error", "line": "enqueue failed", "status": "failed"})
			return
		}
	}
	emit(map[string]string{"stage": "queue", "line": "Queued deployment " + depID.String()})

	seen := 0
	replayLogs := func() {
		dep, err := a.Store.GetDeployment(r.Context(), teamID, depID)
		if err != nil || len(dep.Logs) <= 2 {
			return
		}
		var logs []map[string]any
		if json.Unmarshal(dep.Logs, &logs) != nil {
			return
		}
		for i := seen; i < len(logs); i++ {
			stage, _ := logs[i]["stage"].(string)
			line, _ := logs[i]["line"].(string)
			if line != "" {
				emit(map[string]string{"stage": stage, "line": line})
			}
		}
		seen = len(logs)
	}
	replayLogs()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-ch:
			if ok && len(msg) > 0 {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			}
		case <-ticker.C:
			replayLogs()
			dep, err := a.Store.GetDeployment(r.Context(), teamID, depID)
			if err != nil {
				emit(map[string]string{"stage": "error", "line": err.Error(), "status": "failed"})
				return
			}
			switch dep.Status {
			case "finished":
				emit(map[string]string{"stage": "done", "line": "Deploy finished", "status": "running"})
				return
			case "failed":
				msg := dep.ErrorMessage
				if msg == "" {
					msg = "deploy failed"
				}
				emit(map[string]string{"stage": "error", "line": msg, "status": "failed"})
				return
			case "cancelled":
				emit(map[string]string{"stage": "error", "line": "cancelled", "status": "failed"})
				return
			}
		}
	}
}

func (a *API) handlePatchService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name             *string `json:"name"`
		Description      *string `json:"description"`
		FQDN             *string `json:"fqdn"`
		DockerComposeRaw *string `json:"docker_compose_raw"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	name, desc := svc.Name, svc.Description
	if body.Name != nil {
		name = strings.TrimSpace(*body.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name required")
			return
		}
	}
	if body.Description != nil {
		desc = *body.Description
	}
	updated, err := a.Store.UpdateServiceMeta(r.Context(), teamID, id, name, desc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if body.DockerComposeRaw != nil {
		raw := *body.DockerComposeRaw
		if strings.TrimSpace(raw) == "" {
			writeError(w, http.StatusBadRequest, "docker_compose_raw cannot be empty")
			return
		}
		if _, _, err := services.PrepareCompose(raw, services.PrepareOpts{BaseURL: "http://127.0.0.1"}); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose: %v", err))
			return
		}
		// Clearing the prepared copy forces the next deploy to re-bake networks,
		// magic env, and Traefik labels from this YAML.
		if err := a.Store.UpdateServiceComposeRaw(r.Context(), teamID, id, raw); err != nil {
			mapStoreErr(w, err)
			return
		}
		a.syncResourceComposeEnvRefs(r.Context(), teamID, "service", id, raw)
		updated, err = a.Store.GetService(r.Context(), teamID, id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.FQDN != nil {
		fqdn := proxy.NormalizeDomains(strings.TrimSpace(*body.FQDN))
		if fqdn != "" && !isValidHostnameList(fqdn) {
			writeError(w, http.StatusBadRequest, "invalid fqdn — use hostnames like example.com or https://example.com (comma-separated OK)")
			return
		}
		if err := a.Store.UpdateServiceFQDN(r.Context(), id, fqdn); err != nil {
			mapStoreErr(w, err)
			return
		}
		// Force next deploy to re-bake Traefik labels / SERVICE_URL_* pairs.
		_ = a.Store.UpdateServiceCompose(r.Context(), id, "")
		a.rewriteServiceDomainEnv(r.Context(), teamID, id, fqdn)
		updated, err = a.Store.GetService(r.Context(), teamID, id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, serviceWithLinks(updated))
}

func (a *API) handleStopService(w http.ResponseWriter, r *http.Request) {
	a.runServiceComposeAction(w, r, "stop", "exited")
}

func (a *API) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	opts, ok := a.authorizeDestructiveAction(w, r, svc.Name, true)
	if !ok {
		return
	}

	// Best-effort: tear down compose stack (+ volumes), then remove remote dir.
	if serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc); err == nil {
		if client, err := a.dialServer(r, serverID); err == nil {
			remoteDir := "/data/dockfin/services/" + id.String()
			composePath := remoteDir + "/docker-compose.yml"
			project := "dockfin-svc-" + id.String()[:8]
			downArgs := []string{"docker", "compose", "-p", project, "-f", composePath, "down", "--remove-orphans"}
			if opts.volumes() {
				downArgs = append(downArgs, "-v")
			}
			_, _, _ = sshx.RunArgs(client, downArgs...)
			if opts.configurations() {
				_, _, _ = sshx.RunArgs(client, "rm", "-rf", remoteDir)
			}
			if opts.networks() {
				// Coolify removes a network named after the service UUID; compose down
				// already drops the project default network.
				removeResourceScopedNetwork(client, id.String())
			}
			if opts.dockerCleanup() {
				runDockerCleanup(client)
			}
		}
	}

	if err := a.Store.DeleteService(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleRestartService(w http.ResponseWriter, r *http.Request) {
	a.runServiceComposeAction(w, r, "restart", "running")
}

func (a *API) runServiceComposeAction(w http.ResponseWriter, r *http.Request, action, statusOnOK string) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	serverID, dest, err := a.resolveServiceTarget(r.Context(), teamID, svc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = dest
	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	remoteDir := "/data/dockfin/services/" + id.String()
	composePath := remoteDir + "/docker-compose.yml"
	project := "dockfin-svc-" + id.String()[:8]

	// Cloned / never-deployed services have no compose file on disk.
	if _, _, existsErr := sshx.RunArgs(client, "test", "-f", composePath); existsErr != nil {
		if action == "stop" {
			// Idempotent: nothing running to stop.
			_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
			svc.Status = "exited"
			writeJSON(w, http.StatusOK, serviceWithLinks(svc))
			return
		}
		writeError(w, http.StatusBadRequest, "service has not been deployed yet — deploy first")
		return
	}

	_, errOut, err := sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", composePath, action)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("%s failed: %v %s", action, err, errOut))
		return
	}
	_ = a.Store.UpdateServiceStatus(r.Context(), id, statusOnOK)
	svc.Status = statusOnOK
	writeJSON(w, http.StatusOK, serviceWithLinks(svc))
}

func (a *API) resolveServiceTarget(ctx context.Context, teamID uuid.UUID, svc *store.Service) (uuid.UUID, *store.Destination, error) {
	var serverID uuid.UUID
	var dest *store.Destination
	switch {
	case svc.ServerID != nil:
		serverID = *svc.ServerID
		if svc.DestinationID != nil {
			dest, _ = a.Store.GetDestination(ctx, teamID, *svc.DestinationID)
		}
	case svc.DestinationID != nil:
		var err error
		dest, err = a.Store.GetDestination(ctx, teamID, *svc.DestinationID)
		if err != nil {
			return uuid.Nil, nil, err
		}
		serverID = dest.ServerID
	default:
		return uuid.Nil, nil, fmt.Errorf("service has no server or destination")
	}
	return serverID, dest, nil
}

func (a *API) handleListServiceTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": services.ListTemplates()})
}

func (a *API) handleSentinelMetrics(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServerID         string  `json:"server_id"`
		Token            string  `json:"token"`
		CPUPercent       float64 `json:"cpu_percent"`
		MemoryUsedBytes  int64   `json:"memory_used_bytes"`
		MemoryTotalBytes int64   `json:"memory_total_bytes"`
		DiskUsedBytes    int64   `json:"disk_used_bytes"`
		DiskTotalBytes   int64   `json:"disk_total_bytes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	serverID, err := uuid.Parse(body.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}
	var teamID uuid.UUID
	var expectedToken string
	err = a.Store.Pool.QueryRow(r.Context(), `
		SELECT s.team_id, COALESCE(ss.sentinel_token, '')
		FROM servers s
		LEFT JOIN server_settings ss ON ss.server_id = s.id
		WHERE s.id = $1
	`, serverID).Scan(&teamID, &expectedToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid server or token")
		return
	}
	if expectedToken == "" || body.Token == "" || !secureTokenEqual(expectedToken, body.Token) {
		writeError(w, http.StatusUnauthorized, "invalid server or token")
		return
	}
	_, err = a.Store.Pool.Exec(r.Context(), `
		INSERT INTO server_metrics (team_id, server_id, cpu_percent, memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, teamID, serverID, body.CPUPercent, body.MemoryUsedBytes, body.MemoryTotalBytes, body.DiskUsedBytes, body.DiskTotalBytes)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}

func (a *API) handleListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	var rid *uuid.UUID
	if s := r.URL.Query().Get("resource_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid resource_id")
			return
		}
		rid = &id
	}
	list, err := a.Store.ListScheduledTasks(r.Context(), currentTeamID(r), r.URL.Query().Get("resource_type"), rid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scheduled_tasks": list})
}

func (a *API) handleCreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType  string `json:"resource_type"`
		ResourceID    string `json:"resource_id"`
		Name          string `json:"name"`
		Command       string `json:"command"`
		Frequency     string `json:"frequency"`
		ContainerName string `json:"container_name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rid, err := uuid.Parse(body.ResourceID)
	if err != nil || body.Name == "" || body.Command == "" || body.Frequency == "" {
		writeError(w, http.StatusBadRequest, "resource_id, name, command, frequency required")
		return
	}
	if body.ResourceType == "" {
		body.ResourceType = "application"
	}
	if body.ContainerName != "" && !terminal.ValidContainerName(body.ContainerName) {
		writeError(w, http.StatusBadRequest, "invalid container_name")
		return
	}
	teamID := currentTeamID(r)
	switch body.ResourceType {
	case "application":
		if _, err := a.Store.GetApplication(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	case "database":
		if _, err := a.Store.GetDatabase(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	case "service":
		if _, err := a.Store.GetService(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported resource_type")
		return
	}
	task, err := a.Store.CreateScheduledTask(r.Context(), teamID, body.ResourceType, rid, body.Name, body.Command, body.Frequency, body.ContainerName)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (a *API) handlePatchScheduledTask(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name          *string `json:"name"`
		Command       *string `json:"command"`
		Frequency     *string `json:"frequency"`
		ContainerName *string `json:"container_name"`
		Enabled       *bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.ContainerName != nil && *body.ContainerName != "" && !terminal.ValidContainerName(*body.ContainerName) {
		writeError(w, http.StatusBadRequest, "invalid container_name")
		return
	}
	task, err := a.Store.UpdateScheduledTask(r.Context(), currentTeamID(r), id, store.UpdateScheduledTaskInput{
		Name:      body.Name,
		Command:   body.Command,
		Frequency: body.Frequency,
		Container: body.ContainerName,
		Enabled:   body.Enabled,
	})
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *API) handleDeleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteScheduledTask(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleListScheduledTaskExecutions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetScheduledTask(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	list, err := a.Store.ListScheduledTaskExecutions(r.Context(), teamID, id, 20)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": list})
}

// handleListScheduledTaskExecutionsForTeam returns recent executions across all scheduled tasks
// for the current team, optionally filtered by ?status=failed. Powers the Settings > Scheduled
// Jobs "recent issues" view.
func (a *API) handleListScheduledTaskExecutionsForTeam(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	list, err := a.Store.ListScheduledTaskExecutionsForTeam(r.Context(), currentTeamID(r), status, limit)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": list})
}

func (a *API) handleRunScheduledTask(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	task, err := a.Store.GetScheduledTask(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	execID, err := a.Store.CreateTaskExecution(r.Context(), teamID, task.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	row := store.ScheduledTaskRow{
		ID:           task.ID,
		TeamID:       task.TeamID,
		ResourceType: task.ResourceType,
		ResourceID:   task.ResourceID,
		Name:         task.Name,
		Command:      task.Command,
		Frequency:    task.Frequency,
		Container:    task.Container,
		Enabled:      task.Enabled,
	}
	if a.TaskRunner != nil {
		go a.TaskRunner.ExecuteTaskNow(context.Background(), row, execID)
	} else {
		_ = a.Store.FinishTaskExecution(r.Context(), execID, "failed", "scheduler not configured")
		writeError(w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"execution_id": execID, "status": "running"})
}

func (a *API) handleListServerMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetServer(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	limit := 60
	if s := r.URL.Query().Get("limit"); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := a.Store.ListServerMetrics(r.Context(), teamID, id, limit)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.ServerMetric{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": list})
}

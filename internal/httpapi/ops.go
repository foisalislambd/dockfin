package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/proxy"
	"github.com/goolify/goolify/internal/services"
	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
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
	teamID := currentTeamID(r)
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
		svc.ServerID = &id
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		svc.DestinationID = &id
	}

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
		}
		if svc.FQDN != "" {
			host := strings.TrimSpace(strings.Split(svc.FQDN, ",")[0])
			opts.BaseURL = proxy.PublicURL(host)
			opts.FQDN = host
			opts.RouterName = svc.Name
			opts.ServiceID = svc.ID.String()
		}
	}
	prepared, _, err := services.PrepareCompose(compose, opts)
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
	var serverID uuid.UUID
	var dest *store.Destination
	switch {
	case svc.ServerID != nil:
		serverID = *svc.ServerID
		if svc.DestinationID != nil {
			dest, _ = a.Store.GetDestination(r.Context(), teamID, *svc.DestinationID)
		}
	case svc.DestinationID != nil:
		var err error
		dest, err = a.Store.GetDestination(r.Context(), teamID, *svc.DestinationID)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		serverID = dest.ServerID
	default:
		writeError(w, http.StatusBadRequest, "service has no server or destination")
		return
	}

	stream := r.URL.Query().Get("stream") == "1" ||
		strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	var emit func(stage, line string)
	var finishOK func()
	var finishErr func(msg string)

	if stream {
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

		emit = func(stage, line string) {
			payload, _ := json.Marshal(map[string]string{"stage": stage, "line": line})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
		finishOK = func() {
			payload, _ := json.Marshal(map[string]string{"stage": "done", "line": "Deploy finished", "status": "running"})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
		finishErr = func(msg string) {
			payload, _ := json.Marshal(map[string]string{"stage": "error", "line": msg, "status": "failed"})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	} else {
		emit = func(stage, line string) {}
		finishOK = func() {
			writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
		}
		finishErr = func(msg string) {
			writeError(w, http.StatusInternalServerError, msg)
		}
	}

	emit("prepare", "Preparing docker compose…")
	composeYAML := svc.DockerCompose
	rawCompose := svc.DockerComposeRaw
	if rawCompose == "" {
		rawCompose = composeYAML
	}

	// Assign or repair free domain (never keep *.127.0.0.1.sslip.io — browsers hit localhost).
	if srv, err := a.Store.GetServer(r.Context(), teamID, serverID); err == nil {
		if publicIP := strings.TrimSpace(srv.PublicIP); publicIP == "" || proxy.IsUnusableMagicIP(publicIP) {
			// Best-effort detect before generating domain.
			if client, derr := a.dialServer(r, serverID); derr == nil {
				if detected := detectServerPublicIP(client); detected != "" {
					_ = a.Store.SetServerPublicIP(r.Context(), teamID, serverID, detected)
					srv.PublicIP = detected
				}
			}
		}
		needsFQDN := svc.FQDN == "" || proxy.FQDNUsesUnusableMagicIP(svc.FQDN)
		if needsFQDN {
			if fqdn := generateResourceFQDN(svc.Name, svc.ID, srv); fqdn != "" {
				if fqdn != svc.FQDN {
					emit("prepare", fmt.Sprintf("Updating domain %s → %s", svc.FQDN, fqdn))
				} else {
					emit("prepare", fmt.Sprintf("Assigned free domain %s", fqdn))
				}
				svc.FQDN = fqdn
				_ = a.Store.UpdateServiceFQDN(r.Context(), id, fqdn)
			} else if proxy.FQDNUsesUnusableMagicIP(svc.FQDN) {
				emit("prepare", "Warning: server has no public IP — domain still points at localhost. Set Public IP on the server or run Validate.")
			}
		}
	}

	fqdnHost := strings.TrimSpace(strings.Split(svc.FQDN, ",")[0])
	needPrepare := composeYAML == "" || composeYAML == svc.DockerComposeRaw || looksLikeUnpreparedCompose(svc.DockerComposeRaw, composeYAML)
	if fqdnHost != "" && composeYAML != "" {
		wantURL := proxy.PublicURL(fqdnHost)
		// Re-bake when host missing OR scheme outdated (http→https).
		if !strings.Contains(composeYAML, fqdnHost) || !strings.Contains(composeYAML, wantURL) {
			needPrepare = true
		}
	}
	if proxy.FQDNUsesUnusableMagicIP(composeYAML) {
		needPrepare = true
	}

	if needPrepare {
		opts := services.PrepareOpts{
			ServiceID:   id.String(),
			BaseURL:     "http://127.0.0.1",
			RouterName:  svc.Name,
			ExistingEnv: services.ExtractMagicEnv(composeYAML),
		}
		if dest != nil {
			opts.Network = dest.Network
		}
		if svc.FQDN != "" {
			opts.BaseURL = proxy.PublicURL(svc.FQDN)
			opts.FQDN = fqdnHost
		}
		prepared, _, err := services.PrepareCompose(rawCompose, opts)
		if err != nil {
			if stream {
				finishErr(fmt.Sprintf("prepare compose: %v", err))
			} else {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("prepare compose: %v", err))
			}
			return
		}
		composeYAML = prepared
		_ = a.Store.UpdateServiceCompose(r.Context(), id, prepared)
		if svc.FQDN != "" {
			emit("prepare", fmt.Sprintf("Compose prepared · domain %s", svc.FQDN))
		} else {
			emit("prepare", "Compose prepared (volumes + magic env)")
		}
	} else {
		emit("prepare", "Using stored compose")
		if svc.FQDN != "" {
			emit("prepare", fmt.Sprintf("Public URL: %s", proxy.PublicURL(svc.FQDN)))
		}
	}

	_ = a.Store.UpdateServiceStatus(r.Context(), id, "deploying")
	emit("connect", "Connecting to server over SSH…")
	client, err := a.dialServer(r, serverID)
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		if stream {
			finishErr(err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if dest != nil && dest.Network != "" {
		emit("network", fmt.Sprintf("Ensuring Docker network %q…", dest.Network))
		if _, _, err := sshx.RunArgs(client, "docker", "network", "inspect", dest.Network); err != nil {
			err = sshx.RunArgsStreaming(client, func(line string) { emit("network", line) }, "docker", "network", "create", dest.Network)
			if err != nil {
				_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
				finishErr(fmt.Sprintf("create network: %v", err))
				return
			}
		} else {
			emit("network", "Network already exists")
		}
	}

	remoteDir := "/data/goolify/services/" + id.String()
	emit("setup", fmt.Sprintf("Creating remote dir %s", remoteDir))
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", remoteDir)
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		finishErr(fmt.Sprintf("mkdir: %v %s", err, errOut))
		return
	}

	composePath := remoteDir + "/docker-compose.yml"
	emit("setup", "Writing docker-compose.yml…")
	writeCmd := fmt.Sprintf("cat > %s <<'GOOLIFY_COMPOSE_EOF'\n%s\nGOOLIFY_COMPOSE_EOF", composePath, composeYAML)
	_, errOut, err = sshx.Run(client, writeCmd)
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		finishErr(fmt.Sprintf("write compose: %v %s", err, errOut))
		return
	}
	emit("setup", "Compose file written")

	project := "goolify-svc-" + id.String()[:8]
	emit("compose", fmt.Sprintf("docker compose -p %s up -d --remove-orphans", project))
	err = sshx.RunArgsStreaming(client, func(line string) {
		emit("compose", line)
	}, "docker", "compose", "-p", project, "-f", composePath, "up", "-d", "--remove-orphans")
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		finishErr(fmt.Sprintf("compose up: %v", err))
		return
	}

	_ = a.Store.UpdateServiceStatus(r.Context(), id, "running")
	finishOK()
}

func (a *API) handlePatchService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
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
			remoteDir := "/data/goolify/services/" + id.String()
			composePath := remoteDir + "/docker-compose.yml"
			project := "goolify-svc-" + id.String()[:8]
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
	remoteDir := "/data/goolify/services/" + id.String()
	composePath := remoteDir + "/docker-compose.yml"
	project := "goolify-svc-" + id.String()[:8]
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

func looksLikeUnpreparedCompose(raw, prepared string) bool {
	if prepared == "" || prepared == raw {
		return true
	}
	if strings.Contains(prepared, "$SERVICE_") {
		return true
	}
	if regexp.MustCompile(`(?m)^\s*-\s+SERVICE_(URL|FQDN|PASSWORD|USER)_[A-Z0-9_]+\s*$`).MatchString(prepared) {
		return true
	}
	return hasNamedVolumeMounts(raw) && !hasTopLevelVolumes(prepared)
}

func hasTopLevelVolumes(compose string) bool {
	return regexp.MustCompile(`(?m)^volumes:\s*$`).MatchString(compose) ||
		strings.Contains(compose, "\nvolumes:\n") ||
		strings.HasPrefix(compose, "volumes:\n")
}

func hasNamedVolumeMounts(raw string) bool {
	return regexp.MustCompile(`(?m)^\s*-\s+([a-zA-Z][a-zA-Z0-9_.-]*):/`).MatchString(raw)
}

func (a *API) handleListServiceTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": services.ListTemplates()})
}

func (a *API) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Store.Pool.Query(r.Context(), `
		SELECT id, channel, enabled, events, created_at FROM notification_settings WHERE team_id=$1 ORDER BY channel
	`, currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	defer rows.Close()
	type ns struct {
		ID        uuid.UUID `json:"id"`
		Channel   string    `json:"channel"`
		Enabled   bool      `json:"enabled"`
		Events    []string  `json:"events"`
		CreatedAt time.Time `json:"created_at"`
	}
	var out []ns
	for rows.Next() {
		var n ns
		if err := rows.Scan(&n.ID, &n.Channel, &n.Enabled, &n.Events, &n.CreatedAt); err != nil {
			mapStoreErr(w, err)
			return
		}
		out = append(out, n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (a *API) handleUpsertNotification(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	var body struct {
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
		Events  []string        `json:"events"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(body.Events) == 0 {
		body.Events = []string{"deployment_success", "deployment_failed"}
	}
	cfg := string(body.Config)
	if cfg == "" {
		cfg = "{}"
	}
	enc, err := a.Store.Box.EncryptString(cfg)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	_, err = a.Store.Pool.Exec(r.Context(), `
		INSERT INTO notification_settings (team_id, channel, enabled, config_enc, events)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (team_id, channel) DO UPDATE SET enabled=$3, config_enc=$4, events=$5, updated_at=NOW()
	`, currentTeamID(r), channel, body.Enabled, enc, body.Events)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	if expectedToken == "" || body.Token == "" || body.Token != expectedToken {
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
	q := `
		SELECT id, resource_type, resource_id, name, command, frequency, enabled, created_at
		FROM scheduled_tasks WHERE team_id=$1`
	args := []any{currentTeamID(r)}
	if rt := r.URL.Query().Get("resource_type"); rt != "" {
		args = append(args, rt)
		q += fmt.Sprintf(` AND resource_type=$%d`, len(args))
	}
	if rid := r.URL.Query().Get("resource_id"); rid != "" {
		id, err := uuid.Parse(rid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid resource_id")
			return
		}
		args = append(args, id)
		q += fmt.Sprintf(` AND resource_id=$%d`, len(args))
	}
	q += ` ORDER BY name`
	rows, err := a.Store.Pool.Query(r.Context(), q, args...)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	defer rows.Close()
	type task struct {
		ID           uuid.UUID `json:"id"`
		ResourceType string    `json:"resource_type"`
		ResourceID   uuid.UUID `json:"resource_id"`
		Name         string    `json:"name"`
		Command      string    `json:"command"`
		Frequency    string    `json:"frequency"`
		Enabled      bool      `json:"enabled"`
		CreatedAt    time.Time `json:"created_at"`
	}
	var out []task
	for rows.Next() {
		var t task
		if err := rows.Scan(&t.ID, &t.ResourceType, &t.ResourceID, &t.Name, &t.Command, &t.Frequency, &t.Enabled, &t.CreatedAt); err != nil {
			mapStoreErr(w, err)
			return
		}
		out = append(out, t)
	}
	if out == nil {
		out = []task{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"scheduled_tasks": out})
}

func (a *API) handleCreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		Name         string `json:"name"`
		Command      string `json:"command"`
		Frequency    string `json:"frequency"`
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
	var id uuid.UUID
	err = a.Store.Pool.QueryRow(r.Context(), `
		INSERT INTO scheduled_tasks (team_id, resource_type, resource_id, name, command, frequency)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id
	`, teamID, body.ResourceType, rid, body.Name, body.Command, body.Frequency).Scan(&id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
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

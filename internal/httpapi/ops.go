package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
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
	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"

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
				fqdn = proxy.NormalizeDomains(fqdn)
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
		} else if n := proxy.NormalizeDomains(svc.FQDN); n != "" && n != svc.FQDN {
			// Bare domain.com → https://domain.com (persist scheme for UI + env sync).
			svc.FQDN = n
			_ = a.Store.UpdateServiceFQDN(r.Context(), id, n)
			emit("prepare", fmt.Sprintf("Normalized domain → %s", n))
		}
	}

	fqdnHost := proxy.PrimaryHost(svc.FQDN)
	needPrepare := composeYAML == "" || composeYAML == svc.DockerComposeRaw || looksLikeUnpreparedCompose(svc.DockerComposeRaw, composeYAML)
	if fqdnHost != "" && composeYAML != "" {
		wantURL := proxy.AutoPublicURL(svc.FQDN)
		// Re-bake when host missing OR scheme outdated (http→https for custom domains).
		if !strings.Contains(composeYAML, fqdnHost) || !strings.Contains(composeYAML, wantURL) {
			needPrepare = true
		}
		// Force re-bake when custom domain compose still lacks Let's Encrypt labels.
		if proxy.WantAutoHTTPS(svc.FQDN) && !strings.Contains(composeYAML, "certresolver") {
			needPrepare = true
		}
	}
	if proxy.FQDNUsesUnusableMagicIP(composeYAML) {
		needPrepare = true
	}

	if needPrepare {
		existing := services.ExtractMagicEnv(composeYAML)
		for k, v := range a.loadServiceMagicEnv(r.Context(), teamID, id) {
			existing[k] = v
		}
		opts := services.PrepareOpts{
			ServiceID:   id.String(),
			BaseURL:     "http://127.0.0.1",
			RouterName:  svc.Name + "-" + id.String()[:8],
			ExistingEnv: existing,
		}
		if dest != nil {
			opts.Network = dest.Network
		}
		if u, host := preferURLFromMagicEnv(existing); u != "" {
			opts.BaseURL = u
			// Keep multi-host Domains list when the env URL matches the primary host.
			if host != "" && host == fqdnHost && strings.TrimSpace(svc.FQDN) != "" {
				opts.FQDN = svc.FQDN
			} else {
				opts.FQDN = host
			}
			if host != "" && host != fqdnHost {
				// Environment Variables domain differs — sync resource FQDN with scheme.
				svc.FQDN = proxy.NormalizeDomains(host)
				_ = a.Store.UpdateServiceFQDN(r.Context(), id, svc.FQDN)
				fqdnHost = proxy.PrimaryHost(svc.FQDN)
				emit("prepare", fmt.Sprintf("Using domain from Environment Variables: %s", svc.FQDN))
			}
			// Auto SSL: upgrade sticky http:// custom domains to https:// for Let's Encrypt.
			domainForSSL := opts.FQDN
			if strings.TrimSpace(domainForSSL) == "" {
				domainForSSL = host
			}
			if strings.TrimSpace(svc.FQDN) != "" {
				domainForSSL = svc.FQDN
			}
			if proxy.WantAutoHTTPS(domainForSSL) {
				opts.BaseURL = proxy.AutoPublicURL(domainForSSL)
				opts.FQDN = proxy.NormalizeDomains(domainForSSL)
				if proxy.WantAutoHTTPS(svc.FQDN) && strings.Contains(svc.FQDN, ",") {
					// Keep multi-host list from Domains field.
					opts.FQDN = svc.FQDN
				}
			}
		} else if svc.FQDN != "" {
			opts.BaseURL = proxy.AutoPublicURL(svc.FQDN)
			opts.FQDN = svc.FQDN
		}
		prepared, fullEnv, err := services.PrepareCompose(rawCompose, opts)
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
		a.syncServiceCoolifyEnv(r.Context(), teamID, id, rawCompose, prepared, fullEnv)
		if svc.FQDN != "" {
			emit("prepare", fmt.Sprintf("Compose prepared · domain %s", svc.FQDN))
			if proxy.WantAutoHTTPS(svc.FQDN) {
				emit("prepare", "Auto SSL enabled (Let's Encrypt) for custom domain")
			}
		} else {
			emit("prepare", "Compose prepared (volumes + magic env)")
		}
	} else {
		emit("prepare", "Using stored compose")
		if svc.FQDN != "" {
			emit("prepare", fmt.Sprintf("Public URL: %s", proxy.PublicURL(svc.FQDN)))
		}
		a.syncServiceCoolifyEnv(r.Context(), teamID, id, rawCompose, composeYAML, services.ExtractMagicEnv(composeYAML))
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

	remoteDir := "/data/dockfin/services/" + id.String()
	emit("setup", fmt.Sprintf("Creating remote dir %s", remoteDir))
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", remoteDir)
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		finishErr(fmt.Sprintf("mkdir: %v %s", err, errOut))
		return
	}

	composePath := remoteDir + "/docker-compose.yml"
	emit("setup", "Writing docker-compose.yml…")
	writeCmd := fmt.Sprintf("cat > %s <<'DOCKFIN_COMPOSE_EOF'\n%s\nDOCKFIN_COMPOSE_EOF", composePath, composeYAML)
	_, errOut, err = sshx.Run(client, writeCmd)
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		finishErr(fmt.Sprintf("write compose: %v %s", err, errOut))
		return
	}
	emit("setup", "Compose file written")

	project := "dockfin-svc-" + id.String()[:8]
	upArgs := []string{"docker", "compose", "-p", project, "-f", composePath, "up", "-d", "--remove-orphans"}
	if force {
		upArgs = append(upArgs, "--force-recreate")
		emit("compose", fmt.Sprintf("docker compose -p %s up -d --remove-orphans --force-recreate", project))
	} else {
		emit("compose", fmt.Sprintf("docker compose -p %s up -d --remove-orphans", project))
	}
	err = sshx.RunArgsStreaming(client, func(line string) {
		emit("compose", line)
	}, upArgs...)
	if err != nil {
		_ = a.Store.UpdateServiceStatus(r.Context(), id, "exited")
		finishErr(fmt.Sprintf("compose up: %v", err))
		return
	}

	// compose up returns as soon as containers are started — Traefik often still
	// returns 502 for a few seconds. Wait until the public URL responds so the
	// first browser visit after "Deploy finished" is not a blank/502 page.
	if fqdnHost != "" {
		waitServiceHTTPReady(client, fqdnHost, emit)
	}

	_ = a.Store.UpdateServiceStatus(r.Context(), id, "running")
	finishOK()
}

// waitServiceHTTPReady polls Traefik via Host header until the backend answers
// with a non-gateway error (not 502/503/504) or the deadline expires.
func waitServiceHTTPReady(client *ssh.Client, fqdn string, emit func(stage, line string)) {
	host := proxy.PrimaryHost(fqdn)
	if host == "" {
		host = strings.TrimSpace(strings.Split(fqdn, ",")[0])
	}
	if host == "" || client == nil {
		return
	}
	emit("ready", fmt.Sprintf("Waiting for http://%s to become ready…", host))
	deadline := time.Now().Add(90 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		// Escape host for single-quoted shell arg.
		safeHost := strings.ReplaceAll(host, "'", `'\''`)
		cmd := fmt.Sprintf(
			`curl -sS -o /dev/null -w '%%{http_code}' --connect-timeout 2 --max-time 5 -H 'Host: %s' http://127.0.0.1/ 2>/dev/null || echo 000`,
			safeHost,
		)
		out, _, _ := sshx.Run(client, cmd)
		code := strings.TrimSpace(out)
		if i := strings.LastIndexAny(code, "\n\r"); i >= 0 {
			code = strings.TrimSpace(code[i+1:])
		}
		last = code
		n, _ := strconv.Atoi(code)
		// Ready only when Traefik has a real backend route — not gateway errors
		// and not Traefik's own 404 (router not registered yet).
		if n >= 200 && n < 500 && n != 404 {
			emit("ready", fmt.Sprintf("Service reachable (HTTP %s)", code))
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
	emit("ready", fmt.Sprintf("Timed out waiting for ready (last HTTP %s) — open the URL in a few seconds", last))
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
		FQDN        *string `json:"fqdn"`
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

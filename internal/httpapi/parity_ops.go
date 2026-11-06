package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/containerfiles"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/notify"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/serverhealth"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleGetMaintenance(w http.ResponseWriter, r *http.Request) {
	kind, id, ok := maintenanceIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := a.ownedResource(r, kind, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	c, err := a.Store.GetResourceControl(r.Context(), currentTeamID(r), kind, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) handlePutMaintenance(w http.ResponseWriter, r *http.Request) {
	kind, id, ok := maintenanceIdentity(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.ownedResource(r, kind, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	var body struct {
		Enabled bool   `json:"enabled"`
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	cur, err := a.Store.GetResourceControl(r.Context(), teamID, kind, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	cur.MaintenanceEnabled = body.Enabled
	cur.MaintenanceMessage = strings.TrimSpace(body.Message)
	if err := a.Store.UpsertResourceControl(r.Context(), *cur); err != nil {
		mapStoreErr(w, err)
		return
	}
	serverID, proxyType, network, dialErr := a.resourceServer(r, kind, id)
	warnings := []string{}
	if dialErr != nil || serverID == uuid.Nil {
		if dialErr != nil {
			warnings = append(warnings, dialErr.Error())
		} else {
			warnings = append(warnings, "resource has no server yet")
		}
	} else if strings.EqualFold(proxyType, "none") {
		warnings = append(warnings, "maintenance is saved; this server has no proxy to serve the page")
	} else if client, err := a.dialServer(r, serverID); err != nil {
		warnings = append(warnings, "saved; proxy update waits until the server answers SSH: "+err.Error())
	} else if err := a.syncProxyEdge(r.Context(), client, teamID, serverID, network); err != nil {
		warnings = append(warnings, err.Error())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"maintenance_enabled": cur.MaintenanceEnabled,
		"maintenance_message": cur.MaintenanceMessage,
		"warnings":            warnings,
	})
}

func (a *API) handleServerHealth(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	h, err := a.Store.GetServerHealth(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (a *API) handleCheckServerHealth(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	srv, err := a.Store.GetServer(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	prev, err := a.Store.GetServerHealth(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	ops, _ := a.Store.GetServerOpsSettings(r.Context(), teamID, id)
	threshold := 80
	if ops != nil && ops.DiskUsageNotificationThreshold > 0 {
		threshold = ops.DiskUsageNotificationThreshold
	}
	client, dialErr := a.dialServer(r, id)
	next, notes := serverhealth.Probe(client, *prev, srv.Name, threshold, srv.ProxyType, true, dialErr, time.Now().UTC())
	next.ServerID = id
	next.TeamID = teamID
	if err := a.Store.SaveServerHealth(r.Context(), next); err != nil {
		mapStoreErr(w, err)
		return
	}
	_ = a.Store.UpdateServerReachability(r.Context(), id, next.Reachable)
	warnings := []string{}
	if client != nil && (srv.ProxyType == "" || strings.EqualFold(srv.ProxyType, "traefik")) {
		network := "dockfin"
		if dests, err := a.Store.ListDestinations(r.Context(), teamID, &id); err == nil && len(dests) > 0 && dests[0].Network != "" {
			network = dests[0].Network
		}
		if err := a.syncProxyEdge(r.Context(), client, teamID, id, network); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	for _, n := range notes {
		(&notify.Dispatcher{Store: a.Store}).Send(r.Context(), teamID, notify.Event{Type: n.Type, Title: n.Title, Message: n.Message})
	}
	writeJSON(w, http.StatusOK, map[string]any{"health": next, "notices": notes, "warnings": warnings})
}

func (a *API) handleFlushDatabaseCache(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Confirm != "FLUSH" {
		writeError(w, http.StatusBadRequest, `confirm must be "FLUSH"`)
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "database has no destination")
		return
	}
	enc, err := a.Store.GetDatabaseCredentials(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	password := ""
	if enc != "" && a.Store.Box != nil {
		password, err = a.Store.Box.DecryptString(enc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "decrypt database password")
			return
		}
	}
	args, err := database.CacheFlushArgs(db.Engine, database.ContainerName(db.ID.String()), password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	client, err := a.dialDestination(r, *db.DestinationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		msg := strings.TrimSpace(errOut + " " + out)
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "flushed", "output": strings.TrimSpace(out)})
}

func (a *API) handleListContainerFiles(w http.ResponseWriter, r *http.Request) {
	a.withContainer(w, r, false, func(client *ssh.Client, container, path string) {
		args, err := containerfiles.ListArgs(container, path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		out, errOut, err := sshx.RunArgs(client, args...)
		if err != nil {
			writeError(w, http.StatusBadGateway, strings.TrimSpace(errOut+" "+err.Error()))
			return
		}
		entries := containerfiles.ParseStatLines(out)
		if entries == nil {
			entries = []containerfiles.Entry{}
		}
		if len(entries) > 500 {
			entries = entries[:500]
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": path, "container": container, "entries": entries})
	})
}

func (a *API) handleReadContainerFile(w http.ResponseWriter, r *http.Request) {
	a.withContainer(w, r, false, func(client *ssh.Client, container, path string) {
		sizeArgs, err := containerfiles.SizeArgs(container, path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sizeOut, _, err := sshx.RunArgs(client, sizeArgs...)
		if err != nil {
			writeError(w, http.StatusBadGateway, "cannot stat file")
			return
		}
		n, ok := containerfiles.ParseByteCount(sizeOut)
		if !ok {
			writeError(w, http.StatusBadGateway, "cannot read file size")
			return
		}
		if n > containerfiles.MaxFileBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "file is larger than 512 KiB")
			return
		}
		args, err := containerfiles.ReadArgs(container, path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		out, errOut, err := sshx.RunArgs(client, args...)
		if err != nil {
			writeError(w, http.StatusBadGateway, strings.TrimSpace(errOut+" "+err.Error()))
			return
		}
		if bytes.Contains([]byte(out), []byte{0}) {
			writeError(w, http.StatusUnsupportedMediaType, "file is binary")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": path, "container": container, "size": n, "content": out})
	})
}

func (a *API) handleWriteContainerFile(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		Path      string `json:"path"`
		Container string `json:"container"`
		Content   string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(body.Content) > containerfiles.MaxFileBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "content is larger than 512 KiB")
		return
	}
	a.withContainerBody(w, r, body.Container, body.Path, func(client *ssh.Client, container, path string) {
		if err := writeContainerBytes(client, container, path, []byte(body.Content)); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "written", "path": path, "container": container})
	})
}

func (a *API) handleMkdirContainer(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		Path      string `json:"path"`
		Container string `json:"container"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	a.withContainerBody(w, r, body.Container, body.Path, func(client *ssh.Client, container, path string) {
		args, err := containerfiles.MkdirArgs(container, path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		_, errOut, err := sshx.RunArgs(client, args...)
		if err != nil {
			writeError(w, http.StatusBadGateway, strings.TrimSpace(errOut+" "+err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "created", "path": path})
	})
}

func (a *API) handleDeleteContainerFile(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		Path      string `json:"path"`
		Container string `json:"container"`
		Confirm   string `json:"confirm"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	a.withContainerBody(w, r, body.Container, body.Path, func(client *ssh.Client, container, path string) {
		base := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			base = path[i+1:]
		}
		if body.Confirm != base || base == "" {
			writeError(w, http.StatusBadRequest, "confirm must be the file name")
			return
		}
		args, err := containerfiles.RemoveFileArgs(container, path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		_, errOut, err := sshx.RunArgs(client, args...)
		if err != nil {
			writeError(w, http.StatusBadGateway, strings.TrimSpace(errOut+" "+err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "path": path})
	})
}

func (a *API) handleDetectPublicIP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	v4, err4 := fetchIP(ctx, "https://api.ipify.org")
	v6, err6 := fetchIP(ctx, "https://api6.ipify.org")
	if err4 != nil && err6 != nil {
		writeError(w, http.StatusBadGateway, "could not detect a public address")
		return
	}
	if ip := net.ParseIP(v4); ip == nil || ip.To4() == nil {
		v4 = ""
	}
	if ip := net.ParseIP(v6); ip == nil || ip.To4() != nil {
		v6 = ""
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_ipv4": v4, "public_ipv6": v6})
}

func fetchIP(ctx context.Context, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (a *API) handlePatchApiToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name      *string   `json:"name"`
		Abilities *[]string `json:"abilities"`
		Paused    *bool     `json:"paused"`
		ExpiresIn *int      `json:"expires_in_days"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := ""
	if body.Name != nil {
		name = *body.Name
	}
	var abilities []string
	if body.Abilities != nil {
		abilities = *body.Abilities
	}
	var expires *time.Time
	clear := false
	if body.ExpiresIn != nil {
		if *body.ExpiresIn <= 0 {
			clear = true
		} else {
			t := time.Now().UTC().Add(time.Duration(*body.ExpiresIn) * 24 * time.Hour)
			expires = &t
		}
	}
	tok, err := a.Store.UpdateApiToken(r.Context(), currentUser(r).ID, currentTeamID(r), id, name, abilities, expires, clear, body.Paused)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"api_token": tok})
}

func (a *API) handleRollApiToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	raw, err := crypto.RandomToken(32)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	plain := "dfin_" + raw
	tok, err := a.Store.RollApiToken(r.Context(), currentUser(r).ID, currentTeamID(r), id, plain)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"api_token": tok, "token": plain})
}

func (a *API) syncProxyEdge(ctx context.Context, client *ssh.Client, teamID, serverID uuid.UUID, network string) error {
	targets, err := a.Store.ListServerMaintenance(ctx, teamID, serverID)
	if err != nil {
		return err
	}
	var hosts []proxy.MaintenanceHost
	for _, t := range targets {
		hosts = append(hosts, edgeHosts(t.ResourceID.String(), t.FQDN, t.Message, t.ForceHTTPS)...)
		if t.ResourceType != "service" {
			continue
		}
		domains, err := a.Store.ListServiceDomains(ctx, teamID, t.ResourceID)
		if err != nil {
			continue
		}
		for _, d := range domains {
			hosts = append(hosts, edgeHosts(t.ResourceID.String(), d.Host, t.Message, d.ForceHTTPS)...)
		}
	}
	if network == "" {
		network = "dockfin"
	}
	ops, err := a.Store.GetServerOpsSettings(ctx, teamID, serverID)
	if err != nil {
		return err
	}
	proxyType := "traefik"
	if srv, err := a.Store.GetServer(ctx, teamID, serverID); err == nil && srv.ProxyType != "" {
		proxyType = srv.ProxyType
	}
	return proxy.SyncEdgeWithHosts(client, proxyType, network, hosts, proxy.CatchallConfig{
		Enabled: ops.ProxyCatchallEnabled, URL: ops.ProxyCatchallURL, Permanent: ops.ProxyCatchallPermanent,
	})
}

func edgeHosts(id, raw, message string, https bool) []proxy.MaintenanceHost {
	var out []proxy.MaintenanceHost
	for _, h := range proxy.HostsFromDomainList(raw) {
		out = append(out, proxy.MaintenanceHost{ID: id, Host: h, Message: message, HTTPS: https})
	}
	if len(out) == 0 {
		if h := proxy.HostFromDomainEntry(raw); h != "" {
			out = append(out, proxy.MaintenanceHost{ID: id, Host: h, Message: message, HTTPS: https})
		}
	}
	return out
}

func maintenanceIdentity(r *http.Request) (string, uuid.UUID, bool) {
	if id, err := uuid.Parse(chi.URLParam(r, "appID")); err == nil && strings.Contains(r.URL.Path, "/applications/") {
		return "application", id, true
	}
	if id, err := uuid.Parse(chi.URLParam(r, "serviceID")); err == nil {
		return "service", id, true
	}
	return "", uuid.Nil, false
}

func (a *API) ownedResource(r *http.Request, kind string, id uuid.UUID) (any, error) {
	teamID := currentTeamID(r)
	switch kind {
	case "application":
		return a.Store.GetApplication(r.Context(), teamID, id)
	case "service":
		return a.Store.GetService(r.Context(), teamID, id)
	default:
		return nil, fmt.Errorf("unknown resource")
	}
}

func (a *API) resourceServer(r *http.Request, kind string, id uuid.UUID) (uuid.UUID, string, string, error) {
	teamID := currentTeamID(r)
	switch kind {
	case "application":
		app, err := a.Store.GetApplication(r.Context(), teamID, id)
		if err != nil {
			return uuid.Nil, "", "", err
		}
		if app.DestinationID == nil {
			return uuid.Nil, "", "", fmt.Errorf("application has no destination")
		}
		dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
		if err != nil {
			return uuid.Nil, "", "", err
		}
		srv, err := a.Store.GetServer(r.Context(), teamID, dest.ServerID)
		if err != nil {
			return dest.ServerID, "", dest.Network, err
		}
		return dest.ServerID, srv.ProxyType, dest.Network, nil
	case "service":
		svc, err := a.Store.GetService(r.Context(), teamID, id)
		if err != nil {
			return uuid.Nil, "", "", err
		}
		serverID, dest, err := a.resolveServiceTarget(r.Context(), teamID, svc)
		if err != nil {
			return uuid.Nil, "", "", err
		}
		network := "dockfin"
		if dest != nil && dest.Network != "" {
			network = dest.Network
		}
		srv, err := a.Store.GetServer(r.Context(), teamID, serverID)
		if err != nil {
			return serverID, "", network, err
		}
		return serverID, srv.ProxyType, network, nil
	default:
		return uuid.Nil, "", "", fmt.Errorf("unknown resource")
	}
}

func (a *API) withContainer(w http.ResponseWriter, r *http.Request, _ bool, fn func(*ssh.Client, string, string)) {
	path, err := containerfiles.CleanPath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.withContainerBody(w, r, r.URL.Query().Get("container"), path, fn)
}

func (a *API) withContainerBody(w http.ResponseWriter, r *http.Request, container, rawPath string, fn func(*ssh.Client, string, string)) {
	path, err := containerfiles.CleanPath(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	client, names, err := a.containersForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	container = strings.TrimSpace(container)
	if container == "" {
		if len(names) == 1 {
			container = names[0]
		} else {
			writeError(w, http.StatusBadRequest, "container is required")
			return
		}
	}
	if !containerfiles.ValidContainerName(container) || !containsName(names, container) {
		writeError(w, http.StatusBadRequest, "container is not part of this resource")
		return
	}
	fn(client, container, path)
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func (a *API) containersForRequest(r *http.Request) (*ssh.Client, []string, error) {
	teamID := currentTeamID(r)
	if id, err := uuid.Parse(chi.URLParam(r, "appID")); err == nil && strings.Contains(r.URL.Path, "/applications/") {
		app, err := a.Store.GetApplication(r.Context(), teamID, id)
		if err != nil {
			return nil, nil, err
		}
		if app.DestinationID == nil {
			return nil, nil, fmt.Errorf("application has no destination")
		}
		dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
		if err != nil {
			return nil, nil, err
		}
		client, err := a.dialServer(r, dest.ServerID)
		if err != nil {
			return nil, nil, err
		}
		if app.BuildPack == "dockercompose" {
			project := "dockfin-" + app.ID.String()[:8]
			out, _, err := sshx.RunArgs(client, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}")
			if err != nil {
				return nil, nil, fmt.Errorf("list containers: %w", err)
			}
			return client, splitNames(out), nil
		}
		return client, []string{"dockfin-" + app.ID.String()}, nil
	}
	if id, err := uuid.Parse(chi.URLParam(r, "serviceID")); err == nil && strings.Contains(r.URL.Path, "/services/") {
		svc, err := a.Store.GetService(r.Context(), teamID, id)
		if err != nil {
			return nil, nil, err
		}
		serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc)
		if err != nil {
			return nil, nil, err
		}
		client, err := a.dialServer(r, serverID)
		if err != nil {
			return nil, nil, err
		}
		compose := svc.DockerCompose
		if compose == "" {
			compose = svc.DockerComposeRaw
		}
		return client, listServiceContainerNames(client, id, compose), nil
	}
	if id, err := uuid.Parse(chi.URLParam(r, "dbID")); err == nil {
		db, err := a.Store.GetDatabase(r.Context(), teamID, id)
		if err != nil {
			return nil, nil, err
		}
		if db.DestinationID == nil {
			return nil, nil, fmt.Errorf("database has no destination")
		}
		client, err := a.dialDestination(r, *db.DestinationID)
		if err != nil {
			return nil, nil, err
		}
		return client, []string{database.ContainerName(db.ID.String())}, nil
	}
	return nil, nil, fmt.Errorf("unknown resource")
}

func splitNames(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}

func writeContainerBytes(client *ssh.Client, container, path string, data []byte) error {
	if _, err := containerfiles.CleanPath(path); err != nil {
		return err
	}
	if !containerfiles.ValidContainerName(container) {
		return fmt.Errorf("invalid container")
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdin = bytes.NewReader(data)
	session.Stdout = io.Discard
	var errBuf strings.Builder
	session.Stderr = &errBuf
	cmd := "docker exec -i " + shQuote(container) + " tee -- " + shQuote(path)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("write file: %v %s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isAdmin(r *http.Request) bool {
	role, _ := r.Context().Value(ctxRole).(string)
	return role == "owner" || role == "admin"
}

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type composeServiceDomain struct {
	Domain string `json:"domain"`
}

// handleLoadComposeForApp clones the app repo, reads the compose file, optionally
// prepares a deployable variant, and persists raw + prepared YAML (Coolify Load Compose).
func (a *API) handleLoadComposeForApp(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.BuildPack != "dockercompose" {
		writeError(w, http.StatusBadRequest, "load compose is only for dockercompose build pack")
		return
	}
	if strings.TrimSpace(app.GitRepository) == "" {
		writeError(w, http.StatusBadRequest, "application has no git repository")
		return
	}

	var body struct {
		BaseDirectory         *string `json:"base_directory"`
		DockerComposeLocation *string `json:"docker_compose_location"`
	}
	if err := decodeJSONOptional(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.BaseDirectory != nil {
		app.BaseDirectory = strings.TrimSpace(*body.BaseDirectory)
		if app.BaseDirectory == "" {
			app.BaseDirectory = "/"
		}
	}
	if body.DockerComposeLocation != nil {
		raw := strings.TrimSpace(*body.DockerComposeLocation)
		if raw != "" && raw != "auto" && raw != "auto-detect" {
			norm := services.NormalizeComposeLocation(raw)
			if norm == "" {
				writeError(w, http.StatusBadRequest, "invalid docker_compose_location")
				return
			}
			app.DockerComposeLocation = norm
		} else {
			app.DockerComposeLocation = ""
		}
	}

	result, err := a.loadApplicationCompose(r, teamID, app)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type loadComposeResponse struct {
	Location             string                          `json:"location"`
	BaseDirectory        string                          `json:"base_directory"`
	DockerComposeRaw     string                          `json:"docker_compose_raw"`
	DockerCompose        string                          `json:"docker_compose"`
	DockerComposeDomains map[string]composeServiceDomain `json:"docker_compose_domains"`
	Units                []services.ComposeUnit          `json:"units"`
	Volumes              []services.ComposeVolume        `json:"volumes"`
	Application          *store.Application              `json:"application"`
}

func (a *API) loadApplicationCompose(r *http.Request, teamID uuid.UUID, app *store.Application) (*loadComposeResponse, error) {
	body := detectComposeBody{
		GitRepository: app.GitRepository,
		GitBranch:     app.GitBranch,
	}
	if app.GitSourceID != nil {
		body.GitSourceID = app.GitSourceID.String()
	}
	if app.PrivateKeyID != nil {
		body.PrivateKeyID = app.PrivateKeyID.String()
	}

	repoURL, env, cleanup, err := a.composeDetectCloneAuth(r, teamID, body)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	tmp, err := os.MkdirTemp("", "dockfin-compose-load-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	cleanupDir := tmp
	defer func() { _ = os.RemoveAll(cleanupDir) }()

	ctx := r.Context()
	branch := strings.TrimSpace(app.GitBranch)
	if branch == "" {
		branch = "main"
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, repoURL, tmp)
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		tmp2, err2 := os.MkdirTemp("", "dockfin-compose-load-*")
		if err2 != nil {
			return nil, fmt.Errorf("git clone: %s", truncateOut(out))
		}
		cleanupDir = tmp2
		tmp = tmp2
		cmd = exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, tmp)
		cmd.Env = append(os.Environ(), env...)
		cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return nil, fmt.Errorf("git clone: %s", truncateOut(out2))
		}
	}

	loc := strings.TrimSpace(app.DockerComposeLocation)
	if loc == "" {
		found, err := services.FindComposeFiles(tmp)
		if err != nil {
			return nil, err
		}
		best := services.PreferComposeFile(found)
		if best == "" {
			return nil, fmt.Errorf("no compose file found")
		}
		loc = best
		app.DockerComposeLocation = services.NormalizeComposeLocation(best)
	}

	rel := services.JoinBaseAndComposePath(app.BaseDirectory, loc)
	relFS := strings.TrimPrefix(rel, "/")
	full := filepath.Join(tmp, filepath.FromSlash(relFS))
	rawBytes, err := os.ReadFile(full)
	if err != nil {
		// Fallback: try location alone under repo root (ignore base if file missing).
		alt := strings.TrimPrefix(services.NormalizeComposeLocation(loc), "/")
		altFull := filepath.Join(tmp, filepath.FromSlash(alt))
		rawBytes, err = os.ReadFile(altFull)
		if err != nil {
			return nil, fmt.Errorf("read compose %s: %w", rel, err)
		}
		rel = "/" + alt
		app.DockerComposeLocation = services.NormalizeComposeLocation(alt)
	}
	raw := string(rawBytes)
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("compose file is empty: %s", rel)
	}

	units := services.ParseComposeUnits(raw)
	volumes := services.ParseComposeVolumes(raw)

	domains := parseComposeDomains(app.DockerComposeDomains)
	// Ensure keys exist for non-DB services.
	for _, u := range units {
		if services.IsDatabaseImage(u.Image) {
			continue
		}
		if _, ok := domains[u.Name]; !ok {
			domains[u.Name] = composeServiceDomain{Domain: ""}
		}
	}
	// Seed empty domains from app.FQDN for the first web-like unit.
	if strings.TrimSpace(app.FQDN) != "" {
		assigned := false
		for _, u := range units {
			if services.IsDatabaseImage(u.Image) {
				continue
			}
			if d := domains[u.Name]; strings.TrimSpace(d.Domain) == "" && !assigned {
				domains[u.Name] = composeServiceDomain{Domain: app.FQDN}
				assigned = true
			}
		}
	}

	prepared := raw
	var fullEnv map[string]string
	// Always prepare once so Coolify-style SERVICE_* secrets/URLs are generated for
	// Environment Variables (even when "Compose prepare" is off and we store raw YAML).
	fqdn := aggregateComposeDomains(domains)
	if fqdn == "" {
		fqdn = app.FQDN
	}
	baseURL := proxy.AutoPublicURL(fqdn)
	routerName := app.Name + "-" + app.ID.String()[:8]
	port := services.DetectProxyPortForGitCompose(raw, app.PortsExposes)
	opts := services.PrepareOpts{
		ServiceID:         app.ID.String(),
		BaseURL:           baseURL,
		FQDN:              fqdn,
		RouterName:        routerName,
		Port:              port,
		Redirect:          app.Redirect,
		SkipHTTPSRedirect: !app.IsForceHTTPS,
	}
	if app.DestinationID != nil {
		if dest, err := a.Store.GetDestination(ctx, teamID, *app.DestinationID); err == nil {
			opts.Network = dest.Network
		}
	}
	if envMap, err := a.composeAppExistingEnv(r, teamID, app); err == nil {
		opts.ExistingEnv = envMap
	}
	out, magic, err := services.PrepareCompose(raw, opts)
	if err != nil {
		return nil, fmt.Errorf("prepare compose: %w", err)
	}
	fullEnv = magic
	if app.ComposePrepare {
		prepared = out
	}

	domainJSON, _ := json.Marshal(domains)
	app.DockerComposeRaw = raw
	app.DockerCompose = prepared
	app.DockerComposeDomains = domainJSON
	if app.BaseDirectory == "" {
		app.BaseDirectory = "/"
	}
	if err := a.Store.UpdateApplication(ctx, app); err != nil {
		return nil, err
	}
	a.syncResourceCoolifyEnv(ctx, teamID, "application", app.ID, raw, out, fullEnv)
	a.syncResourceComposeEnvRefs(ctx, teamID, "application", app.ID, raw)
	fresh, err := a.Store.GetApplication(ctx, teamID, app.ID)
	if err != nil {
		return nil, err
	}
	return &loadComposeResponse{
		Location:             fresh.DockerComposeLocation,
		BaseDirectory:        fresh.BaseDirectory,
		DockerComposeRaw:     fresh.DockerComposeRaw,
		DockerCompose:        fresh.DockerCompose,
		DockerComposeDomains: domains,
		Units:                units,
		Volumes:              volumes,
		Application:          fresh,
	}, nil
}

func (a *API) composeAppExistingEnv(r *http.Request, teamID uuid.UUID, app *store.Application) (map[string]string, error) {
	var projectID, envID, serverID *uuid.UUID
	envID = &app.EnvironmentID
	if app.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID); err == nil {
			serverID = &dest.ServerID
		}
	}
	if env, err := a.Store.GetEnvironment(r.Context(), teamID, app.EnvironmentID); err == nil {
		projectID = &env.ProjectID
	}
	return a.Store.ResolvedEnvMap(r.Context(), teamID, "application", app.ID, projectID, envID, serverID)
}

func parseComposeDomains(raw json.RawMessage) map[string]composeServiceDomain {
	out := map[string]composeServiceDomain{}
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func aggregateComposeDomains(domains map[string]composeServiceDomain) string {
	seen := map[string]struct{}{}
	var parts []string
	// Stable order by service name (Go map iteration is random).
	names := make([]string, 0, len(domains))
	for name := range domains {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		d := domains[name]
		v := strings.TrimSpace(d.Domain)
		if v == "" {
			continue
		}
		for _, piece := range strings.Split(v, ",") {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			if _, ok := seen[piece]; ok {
				continue
			}
			seen[piece] = struct{}{}
			parts = append(parts, piece)
		}
	}
	return strings.Join(parts, ",")
}

func enrichApplicationCompose(app *store.Application) map[string]any {
	src := app.DockerComposeRaw
	if src == "" {
		src = app.DockerCompose
	}
	units := services.ParseComposeUnits(src)
	volumes := services.ParseComposeVolumes(src)
	type unitOut struct {
		Name       string `json:"name"`
		Image      string `json:"image"`
		IsDatabase bool   `json:"is_database"`
		Domain     string `json:"domain,omitempty"`
	}
	domains := parseComposeDomains(app.DockerComposeDomains)
	outUnits := make([]unitOut, 0, len(units))
	for _, u := range units {
		uo := unitOut{Name: u.Name, Image: u.Image, IsDatabase: services.IsDatabaseImage(u.Image)}
		if d, ok := domains[u.Name]; ok {
			uo.Domain = d.Domain
		}
		outUnits = append(outUnits, uo)
	}
	b, _ := json.Marshal(app)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["compose_units"] = outUnits
	m["compose_volumes"] = volumes
	m["docker_compose_domains"] = domains
	return m
}

// ensureApplicationComposeEnv generates Coolify SERVICE_* and ${VAR} env vars from an
// already-loaded compose file so Environment Variables fills without re-clicking Load.
func (a *API) ensureApplicationComposeEnv(ctx context.Context, teamID, appID uuid.UUID) {
	app, err := a.Store.GetApplication(ctx, teamID, appID)
	if err != nil || app.BuildPack != "dockercompose" {
		return
	}
	raw := strings.TrimSpace(app.DockerComposeRaw)
	if raw == "" {
		raw = strings.TrimSpace(app.DockerCompose)
	}
	if raw == "" {
		return
	}
	// Always sync ${VAR:-default} style refs (idempotent via KeepValue).
	a.syncResourceComposeEnvRefs(ctx, teamID, "application", appID, raw)

	vars, err := a.Store.ListEnvVars(ctx, teamID, "application", appID, false)
	if err != nil {
		return
	}
	hasService := false
	for _, v := range vars {
		if strings.HasPrefix(v.Key, "SERVICE_") {
			hasService = true
			break
		}
	}
	if hasService {
		return
	}
	// Only run full PrepareCompose when no SERVICE_* magic env exists yet.
	domains := parseComposeDomains(app.DockerComposeDomains)
	fqdn := aggregateComposeDomains(domains)
	if fqdn == "" {
		fqdn = app.FQDN
	}
	opts := services.PrepareOpts{
		ServiceID:         app.ID.String(),
		BaseURL:           proxy.AutoPublicURL(fqdn),
		FQDN:              fqdn,
		RouterName:        app.Name + "-" + app.ID.String()[:8],
		Port:              services.DetectProxyPortForGitCompose(raw, app.PortsExposes),
		Redirect:          app.Redirect,
		SkipHTTPSRedirect: !app.IsForceHTTPS,
	}
	if app.DestinationID != nil {
		if dest, err := a.Store.GetDestination(ctx, teamID, *app.DestinationID); err == nil {
			opts.Network = dest.Network
		}
	}
	if envMap, err := a.Store.ResolvedEnvMap(ctx, teamID, "application", app.ID, nil, &app.EnvironmentID, nil); err == nil {
		opts.ExistingEnv = envMap
	}
	prepared, fullEnv, err := services.PrepareCompose(raw, opts)
	if err != nil {
		return
	}
	a.syncResourceCoolifyEnv(ctx, teamID, "application", app.ID, raw, prepared, fullEnv)
}

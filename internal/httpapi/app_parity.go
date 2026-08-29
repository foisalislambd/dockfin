package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dockfin/dockfin/internal/backup"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleListApplicationBackups(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetApplication(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	list, err := a.Store.ListBackupExecutions(r.Context(), teamID, "application", id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.BackupExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup_executions": list})
}

func (a *API) handleRunApplicationBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	client, err := a.dialDestination(r, *app.DestinationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filename := backup.DefaultAppBackupFilename(id.String())
	path := backup.AppVolumeArchivePath(id.String(), filename)
	exec, err := a.Store.CreateBackupExecution(r.Context(), teamID, "application", id, filename)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	vols, _ := a.Store.ListVolumes(r.Context(), teamID, "application", id)
	var paths []string
	if len(vols) > 0 {
		parent := "/data/dockfin/applications/" + id.String() + "/volumes"
		allUnderParent := true
		var leafs []string
		for _, v := range vols {
			host := strings.TrimSpace(v.HostPath)
			if host == "" {
				host = parent + "/" + v.Name
			}
			leafs = append(leafs, host)
			if !strings.HasPrefix(host, parent+"/") && host != parent {
				allUnderParent = false
			}
		}
		if allUnderParent {
			paths = []string{parent}
		} else {
			paths = leafs
		}
	} else {
		paths = []string{"/data/dockfin/applications/" + id.String() + "/volumes"}
	}
	paths = backup.FilterExistingPaths(client, paths)
	if len(paths) == 0 {
		_ = a.Store.FinishBackupExecution(r.Context(), exec.ID, "failed", 0, "no volume paths found on server")
		writeError(w, http.StatusBadRequest, "no volume paths found on server — deploy first or add volumes")
		return
	}
	if err := backup.TarHostPaths(client, path, paths); err != nil {
		_ = a.Store.FinishBackupExecution(r.Context(), exec.ID, "failed", 0, err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	size := backup.FileSize(client, path)
	_ = a.Store.FinishBackupExecution(r.Context(), exec.ID, "finished", size, "")
	exec.Status = "finished"
	exec.SizeBytes = size
	writeJSON(w, http.StatusOK, exec)
}

func (a *API) handleListAdditionalDestinations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetApplication(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	ids, err := a.Store.ListAdditionalDestinations(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"destination_ids": ids})
}

func (a *API) handleSetAdditionalDestinations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		DestinationIDs []string `json:"destination_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	var destIDs []uuid.UUID
	for _, s := range body.DestinationIDs {
		d, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		destIDs = append(destIDs, d)
	}
	if err := a.Store.SetAdditionalDestinations(r.Context(), teamID, id, destIDs); err != nil {
		mapStoreErr(w, err)
		return
	}
	ids, _ := a.Store.ListAdditionalDestinations(r.Context(), teamID, id)
	if ids == nil {
		ids = []uuid.UUID{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"destination_ids": ids})
}

func (a *API) handleApplicationMetrics(w http.ResponseWriter, r *http.Request) {
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
	if app.DestinationID == nil {
		writeJSON(w, http.StatusOK, map[string]any{"containers": []any{}})
		return
	}
	client, err := a.dialDestination(r, *app.DestinationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	names := a.listAppRunningContainerNames(client, app)
	if len(names) > 12 {
		names = names[:12]
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": dockerStatsOrEmpty("app:"+appID.String(), client, names)})
}

type containerStatRow struct {
	Name     string `json:"name"`
	CPUPerc  string `json:"cpu_percent"`
	MemUsage string `json:"mem_usage"`
	MemPerc  string `json:"mem_percent"`
	NetIO    string `json:"net_io"`
	BlockIO  string `json:"block_io"`
}

type dockerStatsEntry struct {
	at   time.Time
	rows []containerStatRow
}

var (
	dockerStatsCache    sync.Map
	dockerStatsInflight sync.Map // key -> *sync.Mutex
)

const dockerStatsTTL = 25 * time.Second

func dockerStatsOrEmpty(key string, client *ssh.Client, names []string) []containerStatRow {
	rows, err := dockerStatsCached(key, client, names)
	if err != nil {
		return []containerStatRow{}
	}
	return rows
}

func dockerStatsCached(key string, client *ssh.Client, names []string) ([]containerStatRow, error) {
	if v, ok := dockerStatsCache.Load(key); ok {
		e := v.(dockerStatsEntry)
		if time.Since(e.at) < dockerStatsTTL {
			return e.rows, nil
		}
	}
	muI, _ := dockerStatsInflight.LoadOrStore(key, &sync.Mutex{})
	mu := muI.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	if v, ok := dockerStatsCache.Load(key); ok {
		e := v.(dockerStatsEntry)
		if time.Since(e.at) < dockerStatsTTL {
			return e.rows, nil
		}
	}
	rows, err := dockerStats(client, names)
	if err != nil {
		return nil, err
	}
	dockerStatsCache.Store(key, dockerStatsEntry{at: time.Now(), rows: rows})
	return rows, nil
}

// dockerStats runs a one-shot `docker stats` for the given running container names.
func dockerStats(client *ssh.Client, names []string) ([]containerStatRow, error) {
	if client == nil || len(names) == 0 {
		return []containerStatRow{}, nil
	}
	args := append([]string{"docker", "stats", "--no-stream", "--format", "{{json .}}"}, names...)
	out, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return nil, fmt.Errorf("docker stats failed: %v %s", err, strings.TrimSpace(errOut))
	}
	return parseDockerStatsJSON(out), nil
}

func parseDockerStatsJSON(out string) []containerStatRow {
	rows := []containerStatRow{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]any
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		name := dockerStatsField(raw, "Name", "Container", "ID")
		if name == "" {
			continue
		}
		rows = append(rows, containerStatRow{
			Name:     name,
			CPUPerc:  strings.TrimSuffix(dockerStatsField(raw, "CPUPerc"), "%"),
			MemUsage: dockerStatsField(raw, "MemUsage"),
			MemPerc:  strings.TrimSuffix(dockerStatsField(raw, "MemPerc"), "%"),
			NetIO:    dockerStatsField(raw, "NetIO"),
			BlockIO:  dockerStatsField(raw, "BlockIO"),
		})
	}
	return rows
}

func dockerStatsField(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok || v == nil {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func dockerComposeNames(client *ssh.Client, project string, all bool) []string {
	if client == nil || project == "" {
		return nil
	}
	args := []string{"ps"}
	if all {
		args = append(args, "-a")
	}
	args = append(args, "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}")
	out, _, err := sshx.RunArgs(client, append([]string{"docker"}, args...)...)
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}

// handleDatabaseMetrics returns docker stats for the database container.
func (a *API) handleDatabaseMetrics(w http.ResponseWriter, r *http.Request) {
	dbID, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, dbID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.DestinationID == nil {
		writeJSON(w, http.StatusOK, map[string]any{"containers": []any{}})
		return
	}
	client, err := a.dialDestination(r, *db.DestinationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cname := runningDockerName(client, "dockfin-db-"+db.ID.String())
	var names []string
	if cname != "" {
		names = []string{cname}
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": dockerStatsOrEmpty("db:"+dbID.String(), client, names)})
}

func runningDockerName(client *ssh.Client, cname string) string {
	if client == nil || cname == "" {
		return ""
	}
	out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Running}} {{.Name}}", cname)
	if err != nil {
		return ""
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) < 2 || parts[0] != "true" {
		return ""
	}
	return strings.TrimPrefix(parts[1], "/")
}

func (a *API) listAppRunningContainerNames(client *ssh.Client, app *store.Application) []string {
	if app == nil || client == nil {
		return nil
	}
	if app.BuildPack == "dockercompose" {
		return dockerComposeNames(client, "dockfin-"+app.ID.String()[:8], false)
	}
	if name := runningDockerName(client, "dockfin-"+app.ID.String()); name != "" {
		return []string{name}
	}
	return nil
}

func (a *API) dialDestination(r *http.Request, destID uuid.UUID) (*ssh.Client, error) {
	teamID := currentTeamID(r)
	dest, err := a.Store.GetDestination(r.Context(), teamID, destID)
	if err != nil {
		return nil, err
	}
	return a.dialServer(r, dest.ServerID)
}

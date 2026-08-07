package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/backup"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
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
	names := a.listAppContainerNames(client, app)
	if len(names) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"containers": []any{}})
		return
	}
	args := append([]string{"docker", "stats", "--no-stream", "--format", "{{json .}}"}, names...)
	out, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("docker stats failed: %v %s", err, strings.TrimSpace(errOut)))
		return
	}
	type row struct {
		Name     string `json:"name"`
		CPUPerc  string `json:"cpu_percent"`
		MemUsage string `json:"mem_usage"`
		MemPerc  string `json:"mem_percent"`
		NetIO    string `json:"net_io"`
		BlockIO  string `json:"block_io"`
	}
	var containers []row
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]string
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		containers = append(containers, row{
			Name:     raw["Name"],
			CPUPerc:  strings.TrimSuffix(raw["CPUPerc"], "%"),
			MemUsage: raw["MemUsage"],
			MemPerc:  strings.TrimSuffix(raw["MemPerc"], "%"),
			NetIO:    raw["NetIO"],
			BlockIO:  raw["BlockIO"],
		})
	}
	if containers == nil {
		containers = []row{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": containers})
}

func (a *API) listAppContainerNames(client *ssh.Client, app *store.Application) []string {
	if app == nil || client == nil {
		return nil
	}
	var names []string
	if app.BuildPack == "dockercompose" {
		project := "dockfin-" + app.ID.String()[:8]
		out, _, err := sshx.RunArgs(client, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					names = append(names, line)
				}
			}
		}
		return names
	}
	cname := "dockfin-" + app.ID.String()
	if out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.Name}}", cname); err == nil {
		names = append(names, strings.TrimPrefix(strings.TrimSpace(out), "/"))
	} else {
		names = append(names, cname)
	}
	return names
}

func (a *API) dialDestination(r *http.Request, destID uuid.UUID) (*ssh.Client, error) {
	teamID := currentTeamID(r)
	dest, err := a.Store.GetDestination(r.Context(), teamID, destID)
	if err != nil {
		return nil, err
	}
	return a.dialServer(r, dest.ServerID)
}

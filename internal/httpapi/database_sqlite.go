package httpapi

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/storagemount"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleConnectSqliteApplication(w http.ResponseWriter, r *http.Request) {
	dbID, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		ApplicationID string `json:"application_id"`
		MountPath     string `json:"mount_path"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	appID, err := uuid.Parse(strings.TrimSpace(body.ApplicationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "application_id is required")
		return
	}
	mount := strings.TrimSpace(body.MountPath)
	if mount == "" {
		mount = "/var/lib/sqlite"
	}
	if err := storagemount.ValidateMountPath(mount); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, dbID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.Engine != "sqlite" {
		writeError(w, http.StatusBadRequest, "only a sqlite database can be mounted into an application")
		return
	}
	if db.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "choose a destination for the sqlite database first")
		return
	}
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "choose a destination for the application first")
		return
	}
	dbDest, err := a.Store.GetDestination(r.Context(), teamID, *db.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	appDest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if dbDest.ServerID != appDest.ServerID {
		writeError(w, http.StatusBadRequest, "application and sqlite database must be on the same server")
		return
	}
	host := database.HostDataDir(db.ID.String())
	files, err := database.SqliteDatabases(db.EngineConfig)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if app.BuildPack == "dockercompose" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "compose",
			"compose": fmt.Sprintf("    volumes:\n      - %s:%s\n", host, mount),
			"files":   files,
		})
		return
	}
	name := "sqlite-" + db.ID.String()[:8]
	existing, err := a.Store.ListVolumes(r.Context(), teamID, "application", app.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	for _, v := range existing {
		if v.Name == name {
			continue
		}
		if path.Clean(v.MountPath) == mount {
			writeError(w, http.StatusBadRequest, "another mount already uses "+mount)
			return
		}
		if path.Clean(v.HostPath) == host {
			writeError(w, http.StatusBadRequest, "another mount already uses this sqlite directory")
			return
		}
	}
	vol, err := a.Store.UpsertVolume(r.Context(), teamID, "application", app.ID, name, mount, host, false, nil)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "mounted",
		"volume":     vol,
		"host_path":  host,
		"mount_path": mount,
		"files":      files,
	})
}

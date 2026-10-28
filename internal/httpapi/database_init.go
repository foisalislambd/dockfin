package httpapi

import (
	"net/http"

	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleGetDatabaseInitScripts(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	scripts, err := a.Store.DatabaseInitScripts(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if scripts == nil {
		scripts = []store.DatabaseInitScript{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"supported": database.InitSupported(db.Engine),
		"scripts":   scripts,
	})
}

func (a *API) handlePutDatabaseInitScripts(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var body struct {
		Scripts []store.DatabaseInitScript `json:"scripts"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	in := make([]database.InitScript, len(body.Scripts))
	for i, sc := range body.Scripts {
		in[i] = database.InitScript{Filename: sc.Filename, Content: sc.Content}
	}
	normalized, err := database.NormalizeInitScripts(db.Engine, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stored := make([]store.DatabaseInitScript, len(normalized))
	for i, sc := range normalized {
		stored[i] = store.DatabaseInitScript{Filename: sc.Filename, Content: sc.Content}
	}
	if err := a.Store.SetDatabaseInitScripts(r.Context(), teamID, id, stored); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"supported": database.InitSupported(db.Engine),
		"scripts":   stored,
	})
}

func (a *API) loadDatabaseInitScripts(r *http.Request, teamID uuid.UUID, db *store.Database) ([]database.InitScript, error) {
	rows, err := a.Store.DatabaseInitScripts(r.Context(), teamID, db.ID)
	if err != nil {
		return nil, err
	}
	in := make([]database.InitScript, len(rows))
	for i, sc := range rows {
		in[i] = database.InitScript{Filename: sc.Filename, Content: sc.Content}
	}
	return database.NormalizeInitScripts(db.Engine, in)
}

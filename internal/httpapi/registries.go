package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/store"
)

func (a *API) handleListDockerRegistries(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListDockerRegistries(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.DockerRegistry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"docker_registries": list})
}

func (a *API) handleCreateDockerRegistry(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	enc := ""
	if strings.TrimSpace(body.Password) != "" {
		e, err := a.Store.Box.EncryptString(body.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encrypt password")
			return
		}
		enc = e
	}
	reg, err := a.Store.CreateDockerRegistry(r.Context(), currentTeamID(r), body.Name, body.URL, body.Username, enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	reg.PasswordEnc = ""
	writeJSON(w, http.StatusCreated, reg)
}

func (a *API) handleUpdateDockerRegistry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "registryID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name     *string `json:"name"`
		URL      *string `json:"url"`
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	cur, err := a.Store.GetDockerRegistry(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	name, url, user := cur.Name, cur.URL, cur.Username
	if body.Name != nil {
		name = *body.Name
	}
	if body.URL != nil {
		url = *body.URL
	}
	if body.Username != nil {
		user = *body.Username
	}
	var passEnc *string
	if body.Password != nil && strings.TrimSpace(*body.Password) != "" {
		e, err := a.Store.Box.EncryptString(*body.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encrypt password")
			return
		}
		passEnc = &e
	}
	reg, err := a.Store.UpdateDockerRegistry(r.Context(), teamID, id, name, url, user, passEnc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	reg.PasswordEnc = ""
	writeJSON(w, http.StatusOK, reg)
}

func (a *API) handleDeleteDockerRegistry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "registryID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteDockerRegistry(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

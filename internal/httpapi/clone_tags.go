package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/store"
)

func (a *API) handleCloneEnvironment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	result, err := a.Store.CloneEnvironment(r.Context(), currentTeamID(r), pid, eid, body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) handleListTags(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListTags(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": list})
}

func (a *API) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	tag, err := a.Store.CreateTag(r.Context(), currentTeamID(r), body.Name, body.Color)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

func (a *API) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteTag(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleListResourceTags(w http.ResponseWriter, r *http.Request) {
	rt := r.URL.Query().Get("resource_type")
	rid, err := uuid.Parse(r.URL.Query().Get("resource_id"))
	if err != nil || rt == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id required")
		return
	}
	list, err := a.Store.ListResourceTags(r.Context(), currentTeamID(r), rt, rid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": list})
}

func (a *API) handleListEnvironmentTags(w http.ResponseWriter, r *http.Request) {
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetEnvironment(r.Context(), teamID, eid); err != nil {
		mapStoreErr(w, err)
		return
	}
	m, err := a.Store.ListEnvironmentResourceTags(r.Context(), teamID, eid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	// Serialize as array of {resource_type, resource_id, tags}
	type row struct {
		ResourceType string      `json:"resource_type"`
		ResourceID   string      `json:"resource_id"`
		Tags         []store.Tag `json:"tags"`
	}
	out := make([]row, 0, len(m))
	for key, tags := range m {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, row{ResourceType: parts[0], ResourceID: parts[1], Tags: tags})
	}
	writeJSON(w, http.StatusOK, map[string]any{"resource_tags": out})
}

func (a *API) handleAttachTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TagID        string `json:"tag_id"`
		Name         string `json:"name"`
		Color        string `json:"color"`
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := decodeJSON(r, &body); err != nil || body.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id required")
		return
	}
	rid, err := uuid.Parse(body.ResourceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource_id")
		return
	}
	switch body.ResourceType {
	case "application", "database", "service":
	default:
		writeError(w, http.StatusBadRequest, "unsupported resource_type")
		return
	}
	teamID := currentTeamID(r)
	var tagID uuid.UUID
	if body.TagID != "" {
		tagID, err = uuid.Parse(body.TagID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid tag_id")
			return
		}
	} else if strings.TrimSpace(body.Name) != "" {
		tag, err := a.Store.CreateTag(r.Context(), teamID, body.Name, body.Color)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		tagID = tag.ID
	} else {
		writeError(w, http.StatusBadRequest, "tag_id or name required")
		return
	}
	if err := a.Store.AttachTag(r.Context(), teamID, tagID, body.ResourceType, rid); err != nil {
		mapStoreErr(w, err)
		return
	}
	tags, _ := a.Store.ListResourceTags(r.Context(), teamID, body.ResourceType, rid)
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

func (a *API) handleDetachTag(w http.ResponseWriter, r *http.Request) {
	tagID, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}
	rt := r.URL.Query().Get("resource_type")
	rid, err := uuid.Parse(r.URL.Query().Get("resource_id"))
	if err != nil || rt == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id required")
		return
	}
	if err := a.Store.DetachTag(r.Context(), currentTeamID(r), tagID, rt, rid); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

func (a *API) handleMoveResource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType  string `json:"resource_type"`
		ResourceID    string `json:"resource_id"`
		EnvironmentID string `json:"environment_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rt := strings.TrimSpace(body.ResourceType)
	if rt != "application" && rt != "database" && rt != "service" {
		writeError(w, http.StatusBadRequest, "resource_type must be application, database, or service")
		return
	}
	rid, err := uuid.Parse(body.ResourceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource_id")
		return
	}
	eid, err := uuid.Parse(body.EnvironmentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment_id")
		return
	}
	teamID := currentTeamID(r)
	if err := a.assertEnvVarResource(r, teamID, rt, rid); err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Store.MoveResource(r.Context(), teamID, rt, rid, eid); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "moved",
		"resource_type":  rt,
		"resource_id":    rid,
		"environment_id": eid,
	})
}

package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/dockfin/dockfin/internal/store"
)

func writeConflictDetail(w http.ResponseWriter, err error) {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "conflict: ")
	if msg == "" || msg == "conflict" {
		msg = "invalid settings"
	}
	writeError(w, http.StatusBadRequest, msg)
}

func (a *API) handleGetInstanceSettings(w http.ResponseWriter, r *http.Request) {
	st, err := a.Store.GetInstanceSettings(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if st.PublicURL == "" && a.Cfg != nil && a.Cfg.PublicURL != "" {
		st.PublicURL = a.Cfg.PublicURL
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": st})
}

func (a *API) handlePatchInstanceSettings(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var patch store.InstanceSettingsPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, err := a.Store.UpdateInstanceSettings(r.Context(), patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeConflictDetail(w, err)
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": st})
}

func (a *API) handleListOauthSettings(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListOauthSettings(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.OauthSetting{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"oauth_settings": list})
}

func (a *API) handlePatchOauthSetting(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	provider := chi.URLParam(r, "provider")
	var patch store.OauthSettingPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	row, err := a.Store.UpdateOauthSetting(r.Context(), provider, patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeConflictDetail(w, err)
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"oauth_setting": row})
}

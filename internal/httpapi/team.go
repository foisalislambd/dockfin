package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/store"
)

func (a *API) handleListApiTokens(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListApiTokens(r.Context(), currentUser(r).ID, currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"api_tokens": list})
}

func (a *API) handleCreateApiToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string   `json:"name"`
		Abilities []string `json:"abilities"`
		ExpiresIn *int     `json:"expires_in_days"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	plain, err := crypto.RandomToken(32)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	plain = "dfin_" + plain
	var expires *time.Time
	if body.ExpiresIn != nil && *body.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(*body.ExpiresIn) * 24 * time.Hour)
		expires = &t
	}
	tok, err := a.Store.CreateApiToken(r.Context(), currentUser(r).ID, currentTeamID(r), body.Name, plain, body.Abilities, expires)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"api_token": tok,
		"token":     plain,
	})
}

func (a *API) handleDeleteApiToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteApiToken(r.Context(), currentUser(r).ID, currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleListTeamMembers(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListTeamMembers(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": list})
}

func (a *API) handleRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	uid, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.RemoveTeamMember(r.Context(), currentTeamID(r), uid); err != nil {
		if err.Error() == "cannot remove the last owner" {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (a *API) handleListInvitations(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListInvitations(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": list})
}

func (a *API) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	token, err := crypto.RandomToken(24)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	inv, err := a.Store.CreateInvitation(
		r.Context(),
		currentTeamID(r),
		currentUser(r).ID,
		body.Email,
		body.Role,
		token,
		time.Now().UTC().Add(7*24*time.Hour),
	)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (a *API) handleDeleteInvitation(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "inviteID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteInvitation(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}
	user := currentUser(r)
	team, err := a.Store.AcceptInvitation(r.Context(), body.Token, user.ID, user.Email)
	if err != nil {
		if err == store.ErrUnauthorized {
			writeError(w, http.StatusForbidden, "invitation email does not match your account")
			return
		}
		mapStoreErr(w, err)
		return
	}
	// Switch session team if this is a cookie session
	if sess := currentSession(r); sess != nil && sess.ID != uuid.Nil {
		_ = a.Store.SetCurrentTeam(r.Context(), sess.ID, team.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"team": team, "status": "accepted"})
}

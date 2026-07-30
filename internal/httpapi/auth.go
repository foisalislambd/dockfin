package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/goolify/goolify/internal/store"
)

func (a *API) handleRegister(w http.ResponseWriter, r *http.Request) {
	enabled, err := a.Store.RegistrationEnabled(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if !enabled {
		writeError(w, http.StatusForbidden, "registration disabled")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Email == "" || body.Name == "" || len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "email, name, and password (min 8) required")
		return
	}
	hash, err := crypto.HashPassword(body.Password)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	user, team, err := a.Store.CreateUserWithPersonalTeam(r.Context(), body.Email, body.Name, hash)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	token, err := crypto.RandomToken(32)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	expires := time.Now().Add(a.Cfg.SessionTTL)
	if err := a.Store.CreateSession(r.Context(), user.ID, team.ID, token, expires); err != nil {
		mapStoreErr(w, err)
		return
	}
	setSessionCookie(w, a.Cfg, token, expires)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "team": team, "token": token})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, hash, err := a.Store.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !crypto.VerifyPassword(hash, body.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	teams, err := a.Store.ListTeamsForUser(r.Context(), user.ID)
	if err != nil || len(teams) == 0 {
		writeError(w, http.StatusInternalServerError, "no team")
		return
	}
	token, err := crypto.RandomToken(32)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	expires := time.Now().Add(a.Cfg.SessionTTL)
	if err := a.Store.CreateSession(r.Context(), user.ID, teams[0].ID, token, expires); err != nil {
		mapStoreErr(w, err)
		return
	}
	setSessionCookie(w, a.Cfg, token, expires)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "team": teams[0], "teams": teams, "token": token})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if tok := sessionToken(r); tok != "" {
		_ = a.Store.DeleteSession(r.Context(), tok)
	}
	clearSessionCookie(w, a.Cfg)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	sess := currentSession(r)
	teams, err := a.Store.ListTeamsForUser(r.Context(), user.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var current *store.Team
	for i := range teams {
		if sess.CurrentTeamID != nil && teams[i].ID == *sess.CurrentTeamID {
			current = &teams[i]
			break
		}
	}
	if current == nil && len(teams) > 0 {
		current = &teams[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "team": current, "teams": teams})
}

func (a *API) handleSwitchTeam(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TeamID string `json:"team_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID, err := uuid.Parse(body.TeamID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team_id")
		return
	}
	user := currentUser(r)
	if _, err := a.Store.UserRoleOnTeam(r.Context(), user.ID, teamID); err != nil {
		writeError(w, http.StatusForbidden, "not a team member")
		return
	}
	sess := currentSession(r)
	if err := a.Store.SetCurrentTeam(r.Context(), sess.ID, teamID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := a.Store.ListTeamsForUser(r.Context(), currentUser(r).ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

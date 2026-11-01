package httpapi

import "net/http"

func (a *API) handleOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := a.Store.TeamOverview(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

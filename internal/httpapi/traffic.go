package httpapi

import (
	"net/http"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/traffic"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleSentinelTraffic(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServerID string   `json:"server_id"`
		Token    string   `json:"token"`
		Lines    []string `json:"lines"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	serverID, err := uuid.Parse(body.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}
	_, ops, ok := a.sentinelAuthorized(w, r, serverID, body.Token)
	if !ok {
		return
	}
	if !ops.TrafficAnalyticsEnabled {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "disabled"})
		return
	}
	rows := traffic.Sample(traffic.ParseLines(body.Lines), ops.TrafficSampleThreshold)
	if err := a.Store.AddTraffic(r.Context(), serverID, traffic.Bucket(time.Now()), rows, ops.TrafficIPMode, ops.TrafficRetentionDays); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}

func (a *API) handleServerTraffic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetServer(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	a.writeTraffic(w, r, id, nil)
}

func (a *API) handleApplicationTraffic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	hosts, serverID, err := a.Store.ApplicationHosts(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if serverID == uuid.Nil {
		writeJSON(w, http.StatusOK, emptyTraffic())
		return
	}
	a.writeTraffic(w, r, serverID, hosts)
}

func (a *API) handleServiceTraffic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	hosts, serverID, err := a.Store.ServiceHosts(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if serverID == uuid.Nil {
		writeJSON(w, http.StatusOK, emptyTraffic())
		return
	}
	a.writeTraffic(w, r, serverID, hosts)
}

func (a *API) writeTraffic(w http.ResponseWriter, r *http.Request, serverID uuid.UUID, hosts []string) {
	since, trunc := trafficWindow(r.URL.Query().Get("range"))
	report, err := a.Store.TrafficReport(r.Context(), serverID, since, hosts, trunc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	ops, err := a.Store.GetServerOpsSettings(r.Context(), currentTeamID(r), serverID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":  ops.TrafficAnalyticsEnabled,
		"ip_mode":  ops.TrafficIPMode,
		"requests": report.Requests,
		"series":   report.Series,
		"hosts":    report.Hosts,
		"paths":    report.Paths,
		"statuses": report.Statuses,
		"clients":  report.Clients,
	})
}

func trafficWindow(rangeKey string) (time.Time, string) {
	trunc := "hour"
	span := 24 * time.Hour
	switch rangeKey {
	case "7d":
		span = 7 * 24 * time.Hour
		trunc = "day"
	case "30d":
		span = 30 * 24 * time.Hour
		trunc = "day"
	}
	return time.Now().UTC().Add(-span), trunc
}

func emptyTraffic() map[string]any {
	return map[string]any{
		"enabled": false, "ip_mode": "anonymized", "requests": 0,
		"series": []any{}, "hosts": []any{}, "paths": []any{}, "statuses": []any{}, "clients": []any{},
	}
}

func (a *API) sentinelAuthorized(w http.ResponseWriter, r *http.Request, serverID uuid.UUID, token string) (uuid.UUID, *store.ServerOpsSettings, bool) {
	var teamID uuid.UUID
	var expected string
	err := a.Store.Pool.QueryRow(r.Context(), `
		SELECT s.team_id, COALESCE(ss.sentinel_token, '')
		FROM servers s
		LEFT JOIN server_settings ss ON ss.server_id = s.id
		WHERE s.id = $1
	`, serverID).Scan(&teamID, &expected)
	if err != nil || expected == "" || token == "" || !secureTokenEqual(expected, token) {
		writeError(w, http.StatusUnauthorized, "invalid server or token")
		return uuid.Nil, nil, false
	}
	ops, err := a.Store.GetServerOpsSettings(r.Context(), teamID, serverID)
	if err != nil {
		mapStoreErr(w, err)
		return uuid.Nil, nil, false
	}
	return teamID, ops, true
}

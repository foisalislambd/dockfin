package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/services"
	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
)

func (a *API) handleListServices(w http.ResponseWriter, r *http.Request) {
	var envID *uuid.UUID
	if s := r.URL.Query().Get("environment_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		envID = &id
	}
	list, err := a.Store.ListServices(r.Context(), currentTeamID(r), envID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": list})
}

func (a *API) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnvironmentID    string `json:"environment_id"`
		ServerID         string `json:"server_id"`
		DestinationID    string `json:"destination_id"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		ServiceType      string `json:"service_type"`
		DockerComposeRaw string `json:"docker_compose_raw"`
		Template         string `json:"template"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	envID, err := uuid.Parse(body.EnvironmentID)
	if err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "environment_id and name required")
		return
	}
	compose := body.DockerComposeRaw
	svcType := body.ServiceType
	if body.Template != "" {
		tpl, err := services.GetTemplate(body.Template)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		compose = tpl.Compose
		svcType = tpl.Type
	}
	if svcType == "" {
		svcType = "custom"
	}
	svc := &store.Service{
		TeamID: currentTeamID(r), EnvironmentID: envID, Name: body.Name, Description: body.Description,
		ServiceType: svcType, DockerComposeRaw: compose,
	}
	if body.ServerID != "" {
		id, err := uuid.Parse(body.ServerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid server_id")
			return
		}
		svc.ServerID = &id
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		svc.DestinationID = &id
	}
	created, err := a.Store.CreateService(r.Context(), svc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handleGetService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	svc, err := a.Store.GetService(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (a *API) handleDeployService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if svc.DockerComposeRaw == "" {
		writeError(w, http.StatusBadRequest, "service has no docker compose content")
		return
	}
	var serverID uuid.UUID
	switch {
	case svc.ServerID != nil:
		serverID = *svc.ServerID
	case svc.DestinationID != nil:
		dest, err := a.Store.GetDestination(r.Context(), teamID, *svc.DestinationID)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		serverID = dest.ServerID
	default:
		writeError(w, http.StatusBadRequest, "service has no server or destination")
		return
	}
	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	remoteDir := "/data/goolify/services/" + id.String()
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", remoteDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("mkdir: %v %s", err, errOut))
		return
	}
	composePath := remoteDir + "/docker-compose.yml"
	writeCmd := fmt.Sprintf("cat > %s <<'GOOLIFY_COMPOSE_EOF'\n%s\nGOOLIFY_COMPOSE_EOF", composePath, svc.DockerComposeRaw)
	_, errOut, err = sshx.Run(client, writeCmd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("write compose: %v %s", err, errOut))
		return
	}
	project := "goolify-svc-" + id.String()[:8]
	_, errOut, err = sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", composePath, "up", "-d", "--remove-orphans")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compose up: %v %s", err, errOut))
		return
	}
	_ = a.Store.UpdateServiceStatus(r.Context(), id, "running")
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (a *API) handleListServiceTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": services.ListTemplates()})
}

func (a *API) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Store.Pool.Query(r.Context(), `
		SELECT id, channel, enabled, events, created_at FROM notification_settings WHERE team_id=$1 ORDER BY channel
	`, currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	defer rows.Close()
	type ns struct {
		ID        uuid.UUID `json:"id"`
		Channel   string    `json:"channel"`
		Enabled   bool      `json:"enabled"`
		Events    []string  `json:"events"`
		CreatedAt time.Time `json:"created_at"`
	}
	var out []ns
	for rows.Next() {
		var n ns
		if err := rows.Scan(&n.ID, &n.Channel, &n.Enabled, &n.Events, &n.CreatedAt); err != nil {
			mapStoreErr(w, err)
			return
		}
		out = append(out, n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (a *API) handleUpsertNotification(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	var body struct {
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
		Events  []string        `json:"events"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(body.Events) == 0 {
		body.Events = []string{"deployment_success", "deployment_failed"}
	}
	cfg := string(body.Config)
	if cfg == "" {
		cfg = "{}"
	}
	enc, err := a.Store.Box.EncryptString(cfg)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	_, err = a.Store.Pool.Exec(r.Context(), `
		INSERT INTO notification_settings (team_id, channel, enabled, config_enc, events)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (team_id, channel) DO UPDATE SET enabled=$3, config_enc=$4, events=$5, updated_at=NOW()
	`, currentTeamID(r), channel, body.Enabled, enc, body.Events)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleSentinelMetrics(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServerID         string  `json:"server_id"`
		Token            string  `json:"token"`
		CPUPercent       float64 `json:"cpu_percent"`
		MemoryUsedBytes  int64   `json:"memory_used_bytes"`
		MemoryTotalBytes int64   `json:"memory_total_bytes"`
		DiskUsedBytes    int64   `json:"disk_used_bytes"`
		DiskTotalBytes   int64   `json:"disk_total_bytes"`
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
	var teamID uuid.UUID
	var expectedToken string
	err = a.Store.Pool.QueryRow(r.Context(), `
		SELECT s.team_id, COALESCE(ss.sentinel_token, '')
		FROM servers s
		LEFT JOIN server_settings ss ON ss.server_id = s.id
		WHERE s.id = $1
	`, serverID).Scan(&teamID, &expectedToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid server or token")
		return
	}
	if expectedToken == "" || body.Token == "" || body.Token != expectedToken {
		writeError(w, http.StatusUnauthorized, "invalid server or token")
		return
	}
	_, err = a.Store.Pool.Exec(r.Context(), `
		INSERT INTO server_metrics (team_id, server_id, cpu_percent, memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, teamID, serverID, body.CPUPercent, body.MemoryUsedBytes, body.MemoryTotalBytes, body.DiskUsedBytes, body.DiskTotalBytes)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}

func (a *API) handleListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	q := `
		SELECT id, resource_type, resource_id, name, command, frequency, enabled, created_at
		FROM scheduled_tasks WHERE team_id=$1`
	args := []any{currentTeamID(r)}
	if rt := r.URL.Query().Get("resource_type"); rt != "" {
		args = append(args, rt)
		q += fmt.Sprintf(` AND resource_type=$%d`, len(args))
	}
	if rid := r.URL.Query().Get("resource_id"); rid != "" {
		id, err := uuid.Parse(rid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid resource_id")
			return
		}
		args = append(args, id)
		q += fmt.Sprintf(` AND resource_id=$%d`, len(args))
	}
	q += ` ORDER BY name`
	rows, err := a.Store.Pool.Query(r.Context(), q, args...)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	defer rows.Close()
	type task struct {
		ID           uuid.UUID `json:"id"`
		ResourceType string    `json:"resource_type"`
		ResourceID   uuid.UUID `json:"resource_id"`
		Name         string    `json:"name"`
		Command      string    `json:"command"`
		Frequency    string    `json:"frequency"`
		Enabled      bool      `json:"enabled"`
		CreatedAt    time.Time `json:"created_at"`
	}
	var out []task
	for rows.Next() {
		var t task
		if err := rows.Scan(&t.ID, &t.ResourceType, &t.ResourceID, &t.Name, &t.Command, &t.Frequency, &t.Enabled, &t.CreatedAt); err != nil {
			mapStoreErr(w, err)
			return
		}
		out = append(out, t)
	}
	if out == nil {
		out = []task{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"scheduled_tasks": out})
}

func (a *API) handleCreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		Name         string `json:"name"`
		Command      string `json:"command"`
		Frequency    string `json:"frequency"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rid, err := uuid.Parse(body.ResourceID)
	if err != nil || body.Name == "" || body.Command == "" || body.Frequency == "" {
		writeError(w, http.StatusBadRequest, "resource_id, name, command, frequency required")
		return
	}
	if body.ResourceType == "" {
		body.ResourceType = "application"
	}
	teamID := currentTeamID(r)
	switch body.ResourceType {
	case "application":
		if _, err := a.Store.GetApplication(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	case "database":
		if _, err := a.Store.GetDatabase(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	case "service":
		if _, err := a.Store.GetService(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported resource_type")
		return
	}
	var id uuid.UUID
	err = a.Store.Pool.QueryRow(r.Context(), `
		INSERT INTO scheduled_tasks (team_id, resource_type, resource_id, name, command, frequency)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id
	`, teamID, body.ResourceType, rid, body.Name, body.Command, body.Frequency).Scan(&id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) handleListServerMetrics(w http.ResponseWriter, r *http.Request) {
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
	limit := 60
	if s := r.URL.Query().Get("limit"); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := a.Store.ListServerMetrics(r.Context(), teamID, id, limit)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.ServerMetric{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": list})
}

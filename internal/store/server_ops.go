package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ServerOpsSettings holds Coolify-style sentinel / cleanup / edge settings.
type ServerOpsSettings struct {
	SentinelEnabled                     bool            `json:"sentinel_enabled"`
	SentinelToken                       string          `json:"sentinel_token,omitempty"`
	SentinelMetricsRefreshRateSeconds   int             `json:"sentinel_metrics_refresh_rate_seconds"`
	DockerCleanupFrequency              string          `json:"docker_cleanup_frequency"`
	DockerCleanupThreshold              int             `json:"docker_cleanup_threshold"`
	ForceDockerCleanup                  bool            `json:"force_docker_cleanup"`
	CloudflareTunnelToken               string          `json:"cloudflare_tunnel_token,omitempty"`
	CloudflareTunnelEnabled             bool            `json:"cloudflare_tunnel_enabled"`
	LogDrainEnabled                     bool            `json:"log_drain_enabled"`
	LogDrainType                        string          `json:"log_drain_type,omitempty"`
	LogDrainConfig                      string          `json:"log_drain_config,omitempty"`
	CACertificate                       string          `json:"ca_certificate,omitempty"`
	TerminalACLUserIDs                  []string        `json:"terminal_acl_user_ids"`
}

type DockerCleanupExecution struct {
	ID         uuid.UUID  `json:"id"`
	TeamID     uuid.UUID  `json:"team_id"`
	ServerID   uuid.UUID  `json:"server_id"`
	Status     string     `json:"status"`
	Message    string     `json:"message"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ServerProxyConfiguration struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	ServerID  uuid.UUID `json:"server_id"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) GetServerOpsSettings(ctx context.Context, teamID, serverID uuid.UUID) (*ServerOpsSettings, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	var out ServerOpsSettings
	var acl json.RawMessage
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sentinel_enabled,false), COALESCE(sentinel_token,''),
		       COALESCE(sentinel_metrics_refresh_rate_seconds, 30),
		       COALESCE(NULLIF(docker_cleanup_frequency,''),'0 0 * * *'),
		       COALESCE(docker_cleanup_threshold, 80),
		       COALESCE(force_docker_cleanup, false),
		       COALESCE(cloudflare_tunnel_token,''),
		       COALESCE(cloudflare_tunnel_enabled, false),
		       COALESCE(log_drain_enabled, false),
		       COALESCE(log_drain_type,''),
		       COALESCE(log_drain_config,''),
		       COALESCE(ca_certificate,''),
		       COALESCE(terminal_acl_user_ids, '[]'::jsonb)
		FROM server_settings WHERE server_id=$1
	`, serverID).Scan(
		&out.SentinelEnabled, &out.SentinelToken, &out.SentinelMetricsRefreshRateSeconds,
		&out.DockerCleanupFrequency, &out.DockerCleanupThreshold, &out.ForceDockerCleanup,
		&out.CloudflareTunnelToken, &out.CloudflareTunnelEnabled,
		&out.LogDrainEnabled, &out.LogDrainType, &out.LogDrainConfig,
		&out.CACertificate, &acl,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return &ServerOpsSettings{
			DockerCleanupFrequency:            "0 0 * * *",
			DockerCleanupThreshold:            80,
			SentinelMetricsRefreshRateSeconds: 30,
			TerminalACLUserIDs:                []string{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(acl, &out.TerminalACLUserIDs)
	if out.TerminalACLUserIDs == nil {
		out.TerminalACLUserIDs = []string{}
	}
	return &out, nil
}

func (s *Store) SetServerOpsSettings(ctx context.Context, teamID, serverID uuid.UUID, in ServerOpsSettings) error {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return err
	}
	freq := strings.TrimSpace(in.DockerCleanupFrequency)
	if freq == "" {
		freq = "0 0 * * *"
	}
	refresh := in.SentinelMetricsRefreshRateSeconds
	if refresh <= 0 {
		refresh = 30
	}
	thresh := in.DockerCleanupThreshold
	if thresh <= 0 {
		thresh = 80
	}
	acl, _ := json.Marshal(in.TerminalACLUserIDs)
	if len(in.TerminalACLUserIDs) == 0 {
		acl = []byte("[]")
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE server_settings SET
			sentinel_enabled=$2,
			sentinel_token=$3,
			sentinel_metrics_refresh_rate_seconds=$4,
			docker_cleanup_frequency=$5,
			docker_cleanup_threshold=$6,
			force_docker_cleanup=$7,
			cloudflare_tunnel_token=$8,
			cloudflare_tunnel_enabled=$9,
			log_drain_enabled=$10,
			log_drain_type=$11,
			log_drain_config=$12,
			ca_certificate=$13,
			terminal_acl_user_ids=$14,
			updated_at=NOW()
		WHERE server_id=$1
	`, serverID, in.SentinelEnabled, in.SentinelToken, refresh, freq, thresh, in.ForceDockerCleanup,
		in.CloudflareTunnelToken, in.CloudflareTunnelEnabled,
		in.LogDrainEnabled, in.LogDrainType, in.LogDrainConfig,
		in.CACertificate, acl)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateDockerCleanupExecution(ctx context.Context, teamID, serverID uuid.UUID) (*DockerCleanupExecution, error) {
	var e DockerCleanupExecution
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO docker_cleanup_executions (team_id, server_id, status)
		VALUES ($1,$2,'running')
		RETURNING id, team_id, server_id, status, message, started_at, finished_at, created_at
	`, teamID, serverID).Scan(
		&e.ID, &e.TeamID, &e.ServerID, &e.Status, &e.Message, &e.StartedAt, &e.FinishedAt, &e.CreatedAt,
	)
	return &e, err
}

func (s *Store) FinishDockerCleanupExecution(ctx context.Context, id uuid.UUID, status, message string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE docker_cleanup_executions
		SET status=$2, message=$3, finished_at=NOW()
		WHERE id=$1
	`, id, status, message)
	return err
}

func (s *Store) ListDockerCleanupExecutions(ctx context.Context, teamID, serverID uuid.UUID, limit int) ([]DockerCleanupExecution, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, server_id, status, message, started_at, finished_at, created_at
		FROM docker_cleanup_executions
		WHERE team_id=$1 AND server_id=$2
		ORDER BY started_at DESC
		LIMIT $3
	`, teamID, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DockerCleanupExecution
	for rows.Next() {
		var e DockerCleanupExecution
		if err := rows.Scan(&e.ID, &e.TeamID, &e.ServerID, &e.Status, &e.Message, &e.StartedAt, &e.FinishedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListDockerCleanupExecutionsForTeam returns recent docker cleanup executions across all servers
// for a team, optionally filtered by status (e.g. "failed"). Used by the Settings > Scheduled
// Jobs "recent issues" view.
func (s *Store) ListDockerCleanupExecutionsForTeam(ctx context.Context, teamID uuid.UUID, status string, limit int) ([]DockerCleanupExecution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT id, team_id, server_id, status, message, started_at, finished_at, created_at
		FROM docker_cleanup_executions
		WHERE team_id = $1`
	args := []any{teamID}
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY started_at DESC LIMIT $%d", len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DockerCleanupExecution
	for rows.Next() {
		var e DockerCleanupExecution
		if err := rows.Scan(&e.ID, &e.TeamID, &e.ServerID, &e.Status, &e.Message, &e.StartedAt, &e.FinishedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []DockerCleanupExecution{}
	}
	return out, rows.Err()
}

// ListServersDueForDockerCleanup returns servers whose cleanup cron matches minute.
func (s *Store) ListServersForDockerCleanup(ctx context.Context) ([]struct {
	TeamID    uuid.UUID
	ServerID  uuid.UUID
	Frequency string
	Force     bool
}, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT s.team_id, s.id, COALESCE(NULLIF(ss.docker_cleanup_frequency,''),'0 0 * * *'),
		       COALESCE(ss.force_docker_cleanup, false)
		FROM servers s
		JOIN server_settings ss ON ss.server_id = s.id
		WHERE s.is_usable = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		TeamID    uuid.UUID
		ServerID  uuid.UUID
		Frequency string
		Force     bool
	}
	for rows.Next() {
		var row struct {
			TeamID    uuid.UUID
			ServerID  uuid.UUID
			Frequency string
			Force     bool
		}
		if err := rows.Scan(&row.TeamID, &row.ServerID, &row.Frequency, &row.Force); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListServerProxyConfigurations(ctx context.Context, teamID, serverID uuid.UUID) ([]ServerProxyConfiguration, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, server_id, name, value, created_at, updated_at
		FROM server_proxy_configurations
		WHERE team_id=$1 AND server_id=$2
		ORDER BY name
	`, teamID, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServerProxyConfiguration
	for rows.Next() {
		var c ServerProxyConfiguration
		if err := rows.Scan(&c.ID, &c.TeamID, &c.ServerID, &c.Name, &c.Value, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) UpsertServerProxyConfiguration(ctx context.Context, teamID, serverID uuid.UUID, name, value string) (*ServerProxyConfiguration, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name required")
	}
	var c ServerProxyConfiguration
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO server_proxy_configurations (team_id, server_id, name, value)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (server_id, name) DO UPDATE
		SET value=EXCLUDED.value, updated_at=NOW()
		RETURNING id, team_id, server_id, name, value, created_at, updated_at
	`, teamID, serverID, name, value).Scan(
		&c.ID, &c.TeamID, &c.ServerID, &c.Name, &c.Value, &c.CreatedAt, &c.UpdatedAt,
	)
	return &c, err
}

func (s *Store) DeleteServerProxyConfiguration(ctx context.Context, teamID, serverID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM server_proxy_configurations WHERE id=$1 AND team_id=$2 AND server_id=$3
	`, id, teamID, serverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

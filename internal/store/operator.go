package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ResourceControl is per-application or per-service edge behavior that is
// applied without rewriting the resource row: maintenance and permanent www redirects.
type ResourceControl struct {
	TeamID             uuid.UUID `json:"team_id"`
	ResourceType       string    `json:"resource_type"`
	ResourceID         uuid.UUID `json:"resource_id"`
	MaintenanceEnabled bool      `json:"maintenance_enabled"`
	MaintenanceMessage string    `json:"maintenance_message"`
	RedirectPermanent  bool      `json:"redirect_permanent"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (s *Store) GetResourceControl(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) (*ResourceControl, error) {
	var c ResourceControl
	err := s.Pool.QueryRow(ctx, `
		SELECT team_id, resource_type, resource_id, maintenance_enabled, maintenance_message, redirect_permanent, updated_at
		FROM resource_controls
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3
	`, teamID, resourceType, resourceID).Scan(
		&c.TeamID, &c.ResourceType, &c.ResourceID, &c.MaintenanceEnabled, &c.MaintenanceMessage, &c.RedirectPermanent, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return &ResourceControl{TeamID: teamID, ResourceType: resourceType, ResourceID: resourceID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpsertResourceControl(ctx context.Context, c ResourceControl) error {
	msg := strings.TrimSpace(c.MaintenanceMessage)
	if len(msg) > 500 {
		msg = msg[:500]
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO resource_controls (team_id, resource_type, resource_id, maintenance_enabled, maintenance_message, redirect_permanent)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (resource_type, resource_id) DO UPDATE SET
			maintenance_enabled=$4,
			maintenance_message=$5,
			redirect_permanent=$6,
			updated_at=NOW()
	`, c.TeamID, c.ResourceType, c.ResourceID, c.MaintenanceEnabled, msg, c.RedirectPermanent)
	return err
}

// MaintenanceTarget is a resource on one server that should show the maintenance page.
type MaintenanceTarget struct {
	ResourceType string
	ResourceID   uuid.UUID
	FQDN         string
	Message      string
	ForceHTTPS   bool
}

func (s *Store) ListServerMaintenance(ctx context.Context, teamID, serverID uuid.UUID) ([]MaintenanceTarget, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT 'application', a.id, COALESCE(a.fqdn,''), c.maintenance_message, COALESCE(a.is_force_https, TRUE)
		FROM resource_controls c
		JOIN applications a ON a.id = c.resource_id AND a.team_id = c.team_id
		JOIN destinations d ON d.id = a.destination_id
		WHERE c.team_id=$1 AND c.resource_type='application' AND c.maintenance_enabled
		  AND d.server_id=$2
		UNION ALL
		SELECT 'service', sv.id, COALESCE(sv.fqdn,''), c.maintenance_message, COALESCE(sv.is_force_https, TRUE)
		FROM resource_controls c
		JOIN services sv ON sv.id = c.resource_id AND sv.team_id = c.team_id AND sv.deleted_at IS NULL
		LEFT JOIN destinations d ON d.id = sv.destination_id
		WHERE c.team_id=$1 AND c.resource_type='service' AND c.maintenance_enabled
		  AND (sv.server_id=$2 OR d.server_id=$2)
	`, teamID, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MaintenanceTarget
	for rows.Next() {
		var t MaintenanceTarget
		if err := rows.Scan(&t.ResourceType, &t.ResourceID, &t.FQDN, &t.Message, &t.ForceHTTPS); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ListMaintenanceControls(ctx context.Context, teamID uuid.UUID) ([]ResourceControl, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT team_id, resource_type, resource_id, maintenance_enabled, maintenance_message, redirect_permanent, updated_at
		FROM resource_controls
		WHERE team_id=$1 AND maintenance_enabled=TRUE
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceControl
	for rows.Next() {
		var c ResourceControl
		if err := rows.Scan(&c.TeamID, &c.ResourceType, &c.ResourceID, &c.MaintenanceEnabled, &c.MaintenanceMessage, &c.RedirectPermanent, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ServerHealth is the latest reachability, disk, proxy, and patch snapshot.
type ServerHealth struct {
	ServerID             uuid.UUID  `json:"server_id"`
	TeamID               uuid.UUID  `json:"team_id"`
	Reachable            bool       `json:"reachable"`
	ConsecutiveFailures  int        `json:"consecutive_failures"`
	DiskUsedPercent      *int       `json:"disk_used_percent,omitempty"`
	DiskMount            string     `json:"disk_mount"`
	TraefikImage         string     `json:"traefik_image"`
	TraefikOutdated      bool       `json:"traefik_outdated"`
	PatchCount           int        `json:"patch_count"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	LastReachableAt      *time.Time `json:"last_reachable_at,omitempty"`
	LastPatchCheckAt     *time.Time `json:"last_patch_check_at,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	UnreachableNotified  bool       `json:"-"`
	DiskNotified         bool       `json:"-"`
	TraefikNotifiedImage string     `json:"-"`
	PatchNotifiedCount   int        `json:"-"`
}

func (s *Store) GetServerHealth(ctx context.Context, teamID, serverID uuid.UUID) (*ServerHealth, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	h, err := scanServerHealth(s.Pool.QueryRow(ctx, serverHealthSelect+` WHERE server_id=$1 AND team_id=$2`, serverID, teamID))
	if errors.Is(err, pgx.ErrNoRows) {
		return &ServerHealth{ServerID: serverID, TeamID: teamID, DiskMount: "/", PatchNotifiedCount: -1}, nil
	}
	return h, err
}

const serverHealthSelect = `
	SELECT server_id, team_id, reachable, consecutive_failures, disk_used_percent, disk_mount,
	       traefik_image, traefik_outdated, patch_count, last_checked_at, last_reachable_at,
	       last_patch_check_at, last_error, unreachable_notified, disk_notified,
	       traefik_notified_image, patch_notified_count
	FROM server_health`

func scanServerHealth(row pgx.Row) (*ServerHealth, error) {
	var h ServerHealth
	err := row.Scan(
		&h.ServerID, &h.TeamID, &h.Reachable, &h.ConsecutiveFailures, &h.DiskUsedPercent, &h.DiskMount,
		&h.TraefikImage, &h.TraefikOutdated, &h.PatchCount, &h.LastCheckedAt, &h.LastReachableAt,
		&h.LastPatchCheckAt, &h.LastError, &h.UnreachableNotified, &h.DiskNotified,
		&h.TraefikNotifiedImage, &h.PatchNotifiedCount,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *Store) SaveServerHealth(ctx context.Context, h ServerHealth) error {
	if h.DiskMount == "" {
		h.DiskMount = "/"
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO server_health (
			server_id, team_id, reachable, consecutive_failures, disk_used_percent, disk_mount,
			traefik_image, traefik_outdated, patch_count, last_checked_at, last_reachable_at,
			last_patch_check_at, last_error, unreachable_notified, disk_notified,
			traefik_notified_image, patch_notified_count
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (server_id) DO UPDATE SET
			reachable=$3, consecutive_failures=$4, disk_used_percent=$5, disk_mount=$6,
			traefik_image=$7, traefik_outdated=$8, patch_count=$9, last_checked_at=$10,
			last_reachable_at=$11, last_patch_check_at=$12, last_error=$13,
			unreachable_notified=$14, disk_notified=$15, traefik_notified_image=$16,
			patch_notified_count=$17
	`, h.ServerID, h.TeamID, h.Reachable, h.ConsecutiveFailures, h.DiskUsedPercent, h.DiskMount,
		h.TraefikImage, h.TraefikOutdated, h.PatchCount, h.LastCheckedAt, h.LastReachableAt,
		h.LastPatchCheckAt, h.LastError, h.UnreachableNotified, h.DiskNotified,
		h.TraefikNotifiedImage, h.PatchNotifiedCount)
	return err
}

// ServerHealthCandidate is a server the scheduler should probe.
type ServerHealthCandidate struct {
	TeamID           uuid.UUID
	ServerID         uuid.UUID
	Name             string
	ProxyType        string
	DiskThreshold    int
	LastCheckedAt    *time.Time
	LastPatchCheckAt *time.Time
}

func (s *Store) ListServersDueForHealth(ctx context.Context, olderThan time.Duration, limit int) ([]ServerHealthCandidate, error) {
	if limit <= 0 {
		limit = 20
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := s.Pool.Query(ctx, `
		SELECT s.team_id, s.id, s.name, COALESCE(s.proxy_type,'traefik'),
		       COALESCE(ss.disk_usage_notification_threshold, 80),
		       h.last_checked_at, h.last_patch_check_at
		FROM servers s
		LEFT JOIN server_settings ss ON ss.server_id = s.id
		LEFT JOIN server_health h ON h.server_id = s.id
		WHERE h.last_checked_at IS NULL OR h.last_checked_at < $1
		ORDER BY h.last_checked_at NULLS FIRST
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServerHealthCandidate
	for rows.Next() {
		var c ServerHealthCandidate
		if err := rows.Scan(&c.TeamID, &c.ServerID, &c.Name, &c.ProxyType, &c.DiskThreshold, &c.LastCheckedAt, &c.LastPatchCheckAt); err != nil {
			return nil, err
		}
		if c.DiskThreshold <= 0 {
			c.DiskThreshold = 80
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateApiToken changes the description, abilities, expiry, or pause state.
// A nil expiresAt clears expiry when clearExpiry is true.
func (s *Store) UpdateApiToken(ctx context.Context, userID, teamID, tokenID uuid.UUID, name string, abilities []string, expiresAt *time.Time, clearExpiry bool, paused *bool) (*ApiToken, error) {
	cur, err := s.getApiToken(ctx, userID, teamID, tokenID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) != "" {
		cur.Name = strings.TrimSpace(name)
	}
	if len(abilities) > 0 {
		cur.Abilities = abilities
	}
	if clearExpiry {
		cur.ExpiresAt = nil
	} else if expiresAt != nil {
		cur.ExpiresAt = expiresAt
	}
	var pausedAt *time.Time
	if paused != nil {
		if *paused {
			now := time.Now().UTC()
			pausedAt = &now
		}
	} else {
		pausedAt = cur.PausedAt
	}
	var t ApiToken
	err = s.Pool.QueryRow(ctx, `
		UPDATE api_tokens
		SET name=$4, abilities=$5, expires_at=$6, paused_at=$7
		WHERE id=$1 AND user_id=$2 AND team_id=$3
		RETURNING id, user_id, team_id, name, token_prefix, abilities, last_used_at, expires_at, paused_at, created_at
	`, tokenID, userID, teamID, cur.Name, cur.Abilities, cur.ExpiresAt, pausedAt).Scan(
		&t.ID, &t.UserID, &t.TeamID, &t.Name, &t.TokenPrefix, &t.Abilities, &t.LastUsedAt, &t.ExpiresAt, &t.PausedAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

func (s *Store) getApiToken(ctx context.Context, userID, teamID, tokenID uuid.UUID) (*ApiToken, error) {
	var t ApiToken
	err := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, team_id, name, token_prefix, abilities, last_used_at, expires_at, paused_at, created_at
		FROM api_tokens WHERE id=$1 AND user_id=$2 AND team_id=$3
	`, tokenID, userID, teamID).Scan(
		&t.ID, &t.UserID, &t.TeamID, &t.Name, &t.TokenPrefix, &t.Abilities, &t.LastUsedAt, &t.ExpiresAt, &t.PausedAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// RollApiToken replaces the secret and clears a pause. The previous secret stops working.
func (s *Store) RollApiToken(ctx context.Context, userID, teamID, tokenID uuid.UUID, plain string) (*ApiToken, error) {
	prefix := plain
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	var t ApiToken
	err := s.Pool.QueryRow(ctx, `
		UPDATE api_tokens
		SET token_hash=$4, token_prefix=$5, paused_at=NULL, last_used_at=NULL
		WHERE id=$1 AND user_id=$2 AND team_id=$3
		RETURNING id, user_id, team_id, name, token_prefix, abilities, last_used_at, expires_at, paused_at, created_at
	`, tokenID, userID, teamID, HashToken(plain), prefix).Scan(
		&t.ID, &t.UserID, &t.TeamID, &t.Name, &t.TokenPrefix, &t.Abilities, &t.LastUsedAt, &t.ExpiresAt, &t.PausedAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

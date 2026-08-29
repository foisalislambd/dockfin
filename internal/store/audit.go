package store

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID           uuid.UUID  `json:"id"`
	TeamID       uuid.UUID  `json:"team_id"`
	UserID       *uuid.UUID `json:"user_id"`
	UserEmail    string     `json:"user_email,omitempty"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
	Action       string     `json:"action"`
	ResourceType string     `json:"resource_type"`
	ResourceID   string     `json:"resource_id"`
	StatusCode   int        `json:"status_code"`
	IP           string     `json:"ip"`
	UserAgent    string     `json:"user_agent"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Store) InsertAuditLog(ctx context.Context, row AuditLog) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO audit_logs (team_id, user_id, method, path, action, resource_type, resource_id, status_code, ip, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, row.TeamID, row.UserID, row.Method, truncate(row.Path, 500), truncate(row.Action, 120),
		truncate(row.ResourceType, 64), truncate(row.ResourceID, 64), row.StatusCode,
		truncate(row.IP, 64), truncate(row.UserAgent, 240))
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, teamID uuid.UUID, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.team_id, a.user_id, COALESCE(u.email,''), a.method, a.path, a.action,
			a.resource_type, a.resource_id, a.status_code, a.ip, a.user_agent, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id
		WHERE a.team_id=$1
		ORDER BY a.created_at DESC
		LIMIT $2
	`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var r AuditLog
		if err := rows.Scan(
			&r.ID, &r.TeamID, &r.UserID, &r.UserEmail, &r.Method, &r.Path, &r.Action,
			&r.ResourceType, &r.ResourceID, &r.StatusCode, &r.IP, &r.UserAgent, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

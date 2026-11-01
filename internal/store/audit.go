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
	ActorEmail   string     `json:"actor_email,omitempty"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
	Action       string     `json:"action"`
	Category     string     `json:"category"`
	Summary      string     `json:"summary"`
	ResourceType string     `json:"resource_type"`
	ResourceID   string     `json:"resource_id"`
	StatusCode   int        `json:"status_code"`
	IP           string     `json:"ip"`
	UserAgent    string     `json:"user_agent"`
	CreatedAt    time.Time  `json:"created_at"`
}

// AuditQuery filters a team's audit log. IncludeUnscopedAuth also returns
// auth rows that could not be attached to a team (unknown account).
type AuditQuery struct {
	TeamID              uuid.UUID
	Action              string
	Category            string
	ResourceType        string
	Query               string
	Limit               int
	Offset              int
	IncludeUnscopedAuth bool
}

type AuditPage struct {
	Logs   []AuditLog
	Total  int
	Limit  int
	Offset int
}

func (s *Store) InsertAuditLog(ctx context.Context, row AuditLog) error {
	if row.Category == "" {
		row.Category = "mutation"
	}
	var team any
	if row.TeamID != uuid.Nil {
		team = row.TeamID
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO audit_logs (
			team_id, user_id, method, path, action, category, summary, actor_email,
			resource_type, resource_id, status_code, ip, user_agent
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, team, row.UserID, truncate(row.Method, 16), truncate(row.Path, 500), truncate(row.Action, 120),
		truncate(row.Category, 32), truncate(row.Summary, 240), truncate(row.ActorEmail, 240),
		truncate(row.ResourceType, 64), truncate(row.ResourceID, 64), row.StatusCode,
		truncate(row.IP, 64), truncate(row.UserAgent, 240))
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, teamID uuid.UUID, limit int) ([]AuditLog, error) {
	page, err := s.QueryAuditLogs(ctx, AuditQuery{TeamID: teamID, Limit: limit, IncludeUnscopedAuth: true})
	if err != nil {
		return nil, err
	}
	return page.Logs, nil
}

func (s *Store) QueryAuditLogs(ctx context.Context, q AuditQuery) (AuditPage, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	q.Action = strings.TrimSpace(q.Action)
	q.Category = strings.TrimSpace(q.Category)
	q.ResourceType = strings.TrimSpace(q.ResourceType)
	q.Query = escapeLike(strings.TrimSpace(q.Query))
	where := `
		WHERE (
			a.team_id = $1
			OR ($2 AND a.team_id IS NULL AND a.category = 'auth')
		)
		AND ($3 = '' OR a.action = $3)
		AND ($4 = '' OR a.category = $4)
		AND ($5 = '' OR a.resource_type = $5)
		AND (
			$6 = ''
			OR a.summary ILIKE '%' || $6 || '%'
			OR a.actor_email ILIKE '%' || $6 || '%'
			OR COALESCE(u.email, '') ILIKE '%' || $6 || '%'
			OR a.path ILIKE '%' || $6 || '%'
		)`
	args := []any{q.TeamID, q.IncludeUnscopedAuth, q.Action, q.Category, q.ResourceType, q.Query}
	var total int
	if err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id
	`+where, args...).Scan(&total); err != nil {
		return AuditPage{}, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.team_id, a.user_id, COALESCE(u.email,''), COALESCE(a.actor_email,''),
			a.method, a.path, a.action, COALESCE(a.category,''), COALESCE(a.summary,''),
			a.resource_type, a.resource_id, a.status_code, a.ip, a.user_agent, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id
	`+where+`
		ORDER BY a.created_at DESC
		LIMIT $7 OFFSET $8
	`, append(args, q.Limit, q.Offset)...)
	if err != nil {
		return AuditPage{}, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var r AuditLog
		var teamID *uuid.UUID
		if err := rows.Scan(
			&r.ID, &teamID, &r.UserID, &r.UserEmail, &r.ActorEmail, &r.Method, &r.Path, &r.Action,
			&r.Category, &r.Summary, &r.ResourceType, &r.ResourceID, &r.StatusCode, &r.IP, &r.UserAgent, &r.CreatedAt,
		); err != nil {
			return AuditPage{}, err
		}
		if teamID != nil {
			r.TeamID = *teamID
		}
		if r.ActorEmail == "" {
			r.ActorEmail = r.UserEmail
		}
		out = append(out, r)
	}
	if out == nil {
		out = []AuditLog{}
	}
	return AuditPage{Logs: out, Total: total, Limit: q.Limit, Offset: q.Offset}, rows.Err()
}

// PruneAuditLogs deletes rows older than the retention window. A non-positive
// number keeps the log forever.
func (s *Store) PruneAuditLogs(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM audit_logs
		WHERE created_at < NOW() - ($1::int * INTERVAL '1 day')
	`, retentionDays)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

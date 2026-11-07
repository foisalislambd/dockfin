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

type ProjectGrant struct {
	ProjectID  uuid.UUID `json:"project_id"`
	Permission string    `json:"permission"`
}

type AccessScope struct {
	Unrestricted bool
	Grants       map[uuid.UUID]string
	EnvProject   map[uuid.UUID]uuid.UUID
}

func (a AccessScope) Allows(projectID uuid.UUID, write, del bool) bool {
	if a.Unrestricted {
		return true
	}
	perm := a.Grants[projectID]
	switch perm {
	case "manage":
		return true
	case "deploy":
		return !del
	case "view":
		return !write && !del
	default:
		return false
	}
}

func (s *Store) MemberAccess(ctx context.Context, teamID, userID uuid.UUID, role string) (AccessScope, error) {
	out := AccessScope{Grants: map[uuid.UUID]string{}, EnvProject: map[uuid.UUID]uuid.UUID{}}
	if role == "owner" || role == "admin" {
		out.Unrestricted = true
		return out, nil
	}
	var scope string
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(access_scope,'all') FROM team_members WHERE team_id=$1 AND user_id=$2`, teamID, userID).Scan(&scope)
	if err != nil {
		return out, err
	}
	if scope != "projects" {
		out.Unrestricted = true
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT project_id, permission FROM project_acl WHERE team_id=$1 AND user_id=$2`, teamID, userID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var perm string
		if err := rows.Scan(&id, &perm); err != nil {
			return out, err
		}
		out.Grants[id] = perm
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	envs, err := s.Pool.Query(ctx, `SELECT id, project_id FROM environments WHERE team_id=$1`, teamID)
	if err != nil {
		return out, err
	}
	defer envs.Close()
	for envs.Next() {
		var id, projectID uuid.UUID
		if err := envs.Scan(&id, &projectID); err != nil {
			return out, err
		}
		out.EnvProject[id] = projectID
	}
	return out, envs.Err()
}

func (s *Store) SetProjectAccess(ctx context.Context, teamID, userID uuid.UUID, scope string, grants []ProjectGrant) error {
	if scope != "all" && scope != "projects" {
		return fmt.Errorf("access_scope must be all or projects")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var role string
	err = tx.QueryRow(ctx, `SELECT role FROM team_members WHERE team_id=$1 AND user_id=$2 FOR UPDATE`, teamID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role == "owner" {
		return fmt.Errorf("owners always have access to every project")
	}
	if _, err := tx.Exec(ctx, `UPDATE team_members SET access_scope=$3 WHERE team_id=$1 AND user_id=$2`, teamID, userID, scope); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM project_acl WHERE team_id=$1 AND user_id=$2`, teamID, userID); err != nil {
		return err
	}
	if scope == "projects" {
		for _, g := range grants {
			switch g.Permission {
			case "view", "deploy", "manage":
			default:
				return fmt.Errorf("permission must be view, deploy, or manage")
			}
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT true FROM projects WHERE id=$1 AND team_id=$2`, g.ProjectID, teamID).Scan(&ok); err != nil {
				return fmt.Errorf("project is not on this team")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO project_acl (team_id, user_id, project_id, permission) VALUES ($1,$2,$3,$4)`, teamID, userID, g.ProjectID, g.Permission); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListProjectGrants(ctx context.Context, teamID, userID uuid.UUID) ([]ProjectGrant, error) {
	rows, err := s.Pool.Query(ctx, `SELECT project_id, permission FROM project_acl WHERE team_id=$1 AND user_id=$2 ORDER BY project_id`, teamID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectGrant
	for rows.Next() {
		var g ProjectGrant
		if err := rows.Scan(&g.ProjectID, &g.Permission); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if out == nil {
		out = []ProjectGrant{}
	}
	return out, rows.Err()
}

func (s *Store) ProjectIDForResource(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID) (uuid.UUID, bool, error) {
	var q string
	switch kind {
	case "project":
		q = `SELECT id FROM projects WHERE team_id=$1 AND id=$2`
		var projectID uuid.UUID
		err := s.Pool.QueryRow(ctx, q, teamID, id).Scan(&projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return projectID, err == nil, err
	case "application":
		q = `SELECT e.project_id FROM applications a JOIN environments e ON e.id=a.environment_id WHERE a.team_id=$1 AND a.id=$2`
	case "service":
		q = `SELECT e.project_id FROM services sv JOIN environments e ON e.id=sv.environment_id WHERE sv.team_id=$1 AND sv.id=$2 AND sv.deleted_at IS NULL`
	case "database":
		q = `SELECT e.project_id FROM databases d JOIN environments e ON e.id=d.environment_id WHERE d.team_id=$1 AND d.id=$2`
	case "environment":
		q = `SELECT project_id FROM environments WHERE team_id=$1 AND id=$2`
	case "deployment":
		q = `SELECT e.project_id FROM deployments dep
			LEFT JOIN applications a ON a.id=dep.application_id
			LEFT JOIN services sv ON sv.id=dep.service_id
			JOIN environments e ON e.id=COALESCE(a.environment_id, sv.environment_id)
			WHERE dep.team_id=$1 AND dep.id=$2`
	default:
		return uuid.Nil, false, nil
	}
	var projectID uuid.UUID
	err := s.Pool.QueryRow(ctx, q, teamID, id).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return projectID, true, nil
}

type FleetRow struct {
	ServerID         uuid.UUID  `json:"server_id"`
	Name             string     `json:"name"`
	IP               string     `json:"ip"`
	Reachable        bool       `json:"reachable"`
	ProxyType        string     `json:"proxy_type"`
	ProxyStatus      string     `json:"proxy_status"`
	CPUPercent       *float64   `json:"cpu_percent"`
	MemoryUsedBytes  *int64     `json:"memory_used_bytes"`
	MemoryTotalBytes *int64     `json:"memory_total_bytes"`
	DiskUsedBytes    *int64     `json:"disk_used_bytes"`
	DiskTotalBytes   *int64     `json:"disk_total_bytes"`
	RecordedAt       *time.Time `json:"recorded_at"`
}

func (s *Store) FleetMetrics(ctx context.Context, teamID uuid.UUID) ([]FleetRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT s.id, s.name, s.ip, s.is_reachable, s.proxy_type, s.proxy_status,
		       m.cpu_percent, m.memory_used_bytes, m.memory_total_bytes, m.disk_used_bytes, m.disk_total_bytes, m.recorded_at
		FROM servers s
		LEFT JOIN LATERAL (
			SELECT cpu_percent, memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes, recorded_at
			FROM server_metrics
			WHERE server_id=s.id
			ORDER BY recorded_at DESC
			LIMIT 1
		) m ON true
		WHERE s.team_id=$1
		ORDER BY s.name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FleetRow
	for rows.Next() {
		var row FleetRow
		if err := rows.Scan(&row.ServerID, &row.Name, &row.IP, &row.Reachable, &row.ProxyType, &row.ProxyStatus,
			&row.CPUPercent, &row.MemoryUsedBytes, &row.MemoryTotalBytes, &row.DiskUsedBytes, &row.DiskTotalBytes, &row.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if out == nil {
		out = []FleetRow{}
	}
	return out, rows.Err()
}

func (s *Store) ServerResourceCount(ctx context.Context, serverID uuid.UUID) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM applications a JOIN destinations d ON d.id=a.destination_id WHERE d.server_id=$1)
		+ (SELECT COUNT(*) FROM services sv WHERE sv.deleted_at IS NULL AND (sv.server_id=$1 OR sv.destination_id IN (SELECT id FROM destinations WHERE server_id=$1)))
		+ (SELECT COUNT(*) FROM databases db JOIN destinations d ON d.id=db.destination_id WHERE d.server_id=$1)
	`, serverID).Scan(&n)
	return n, err
}

// TransferServer moves a server, its destinations, and the resources that run
// on it to another team. A project that also has resources on another server
// stays put; those resources are copied into a new project on the target team.
// The private key is copied so the source team keeps its own copy.
func (s *Store) TransferServer(ctx context.Context, sourceTeam, targetTeam, serverID uuid.UUID) (*TransferResult, error) {
	return s.transferServer(ctx, sourceTeam, targetTeam, serverID)
}

type ServerExport struct {
	Version      int          `json:"version"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	IP           string       `json:"ip"`
	Port         int          `json:"port"`
	UserName     string       `json:"user_name"`
	ProxyType    string       `json:"proxy_type"`
	Destinations []ExportDest `json:"destinations"`
}

type ExportDest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Network string `json:"network"`
}

func (s *Store) ExportServer(ctx context.Context, teamID, serverID uuid.UUID) (*ServerExport, error) {
	var exp ServerExport
	exp.Version = 1
	err := s.Pool.QueryRow(ctx, `SELECT name, description, ip, port, user_name, proxy_type FROM servers WHERE id=$1 AND team_id=$2`, serverID, teamID).
		Scan(&exp.Name, &exp.Description, &exp.IP, &exp.Port, &exp.UserName, &exp.ProxyType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT name, kind, network FROM destinations WHERE server_id=$1 AND team_id=$2`, serverID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d ExportDest
		if err := rows.Scan(&d.Name, &d.Kind, &d.Network); err != nil {
			return nil, err
		}
		exp.Destinations = append(exp.Destinations, d)
	}
	if exp.Destinations == nil {
		exp.Destinations = []ExportDest{}
	}
	return &exp, rows.Err()
}

func (s *Store) ImportServer(ctx context.Context, teamID uuid.UUID, keyID *uuid.UUID, exp ServerExport) (*Server, error) {
	if exp.Version != 1 || strings.TrimSpace(exp.Name) == "" || strings.TrimSpace(exp.IP) == "" {
		return nil, fmt.Errorf("server bundle is incomplete")
	}
	if exp.Port == 0 {
		exp.Port = 22
	}
	if exp.UserName == "" {
		exp.UserName = "root"
	}
	if exp.ProxyType == "" {
		exp.ProxyType = "traefik"
	}
	srv, err := s.CreateServer(ctx, teamID, keyID, exp.Name, exp.Description, exp.IP, exp.UserName, exp.Port, exp.ProxyType)
	if err != nil {
		return nil, err
	}
	for _, d := range exp.Destinations {
		if d.Network == "" || d.Network == "dockfin" {
			continue
		}
		if d.Kind == "" {
			d.Kind = "standalone"
		}
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO destinations (team_id, server_id, name, kind, network)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (server_id, network) DO NOTHING
		`, teamID, srv.ID, d.Name, d.Kind, d.Network)
	}
	return srv, nil
}

func (s *Store) ApplicationsForRepo(ctx context.Context, teamID uuid.UUID, repoPath string) ([]uuid.UUID, error) {
	repoPath = strings.ToLower(strings.Trim(strings.TrimSpace(repoPath), "/"))
	if repoPath == "" || strings.ContainsAny(repoPath, "%_") {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id FROM applications
		WHERE team_id=$1 AND lower(git_repository) LIKE '%' || $2 || '%'
	`, teamID, repoPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type GitLabSystemHook struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	GitLabURL string    `json:"gitlab_url"`
	HookID    *int64    `json:"hook_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) CreateGitLabSystemHook(ctx context.Context, teamID uuid.UUID, gitlabURL, tokenHash, tokenEnc string, hookID *int64) (*GitLabSystemHook, error) {
	var h GitLabSystemHook
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO gitlab_system_hooks (team_id, gitlab_url, token_hash, token_enc, hook_id)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, team_id, gitlab_url, hook_id, created_at
	`, teamID, gitlabURL, tokenHash, tokenEnc, hookID).Scan(&h.ID, &h.TeamID, &h.GitLabURL, &h.HookID, &h.CreatedAt)
	return &h, err
}

func (s *Store) GitLabSystemHookByToken(ctx context.Context, tokenHash string) (*GitLabSystemHook, string, error) {
	var h GitLabSystemHook
	var enc string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, gitlab_url, hook_id, created_at, token_enc
		FROM gitlab_system_hooks WHERE token_hash=$1
	`, tokenHash).Scan(&h.ID, &h.TeamID, &h.GitLabURL, &h.HookID, &h.CreatedAt, &enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &h, enc, err
}

func (s *Store) ListGitLabSystemHooks(ctx context.Context, teamID uuid.UUID) ([]GitLabSystemHook, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, team_id, gitlab_url, hook_id, created_at FROM gitlab_system_hooks WHERE team_id=$1 ORDER BY created_at`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitLabSystemHook
	for rows.Next() {
		var h GitLabSystemHook
		if err := rows.Scan(&h.ID, &h.TeamID, &h.GitLabURL, &h.HookID, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if out == nil {
		out = []GitLabSystemHook{}
	}
	return out, rows.Err()
}

func (s *Store) GitLabSystemHook(ctx context.Context, teamID, id uuid.UUID) (*GitLabSystemHook, string, error) {
	var h GitLabSystemHook
	var enc string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, gitlab_url, hook_id, created_at, token_enc
		FROM gitlab_system_hooks WHERE team_id=$1 AND id=$2
	`, teamID, id).Scan(&h.ID, &h.TeamID, &h.GitLabURL, &h.HookID, &h.CreatedAt, &enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &h, enc, err
}

func (s *Store) DeleteGitLabSystemHook(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM gitlab_system_hooks WHERE team_id=$1 AND id=$2`, teamID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type Passkey struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"user_id"`
	Name         string          `json:"name"`
	CredentialID []byte          `json:"-"`
	Credential   json.RawMessage `json:"-"`
	CreatedAt    time.Time       `json:"created_at"`
	LastUsedAt   *time.Time      `json:"last_used_at,omitempty"`
}

func (s *Store) PasskeysEnabled(ctx context.Context) (bool, error) {
	var on bool
	err := s.Pool.QueryRow(ctx, `SELECT enabled FROM passkey_settings WHERE id=1`).Scan(&on)
	return on, err
}

func (s *Store) SetPasskeysEnabled(ctx context.Context, on bool) error {
	_, err := s.Pool.Exec(ctx, `UPDATE passkey_settings SET enabled=$1 WHERE id=1`, on)
	return err
}

func (s *Store) ListPasskeys(ctx context.Context, userID uuid.UUID) ([]Passkey, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, user_id, name, credential_id, credential, created_at, last_used_at FROM passkeys WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Passkey
	for rows.Next() {
		var p Passkey
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.CredentialID, &p.Credential, &p.CreatedAt, &p.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []Passkey{}
	}
	return out, rows.Err()
}

func (s *Store) SavePasskey(ctx context.Context, userID uuid.UUID, name string, credentialID []byte, credential json.RawMessage) (*Passkey, error) {
	if strings.TrimSpace(name) == "" {
		name = "Passkey"
	}
	var p Passkey
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO passkeys (user_id, name, credential_id, credential)
		VALUES ($1,$2,$3,$4)
		RETURNING id, user_id, name, credential_id, credential, created_at, last_used_at
	`, userID, name, credentialID, credential).Scan(&p.ID, &p.UserID, &p.Name, &p.CredentialID, &p.Credential, &p.CreatedAt, &p.LastUsedAt)
	return &p, err
}

func (s *Store) RenamePasskey(ctx context.Context, userID, id uuid.UUID, name string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE passkeys SET name=$3 WHERE user_id=$1 AND id=$2`, userID, id, strings.TrimSpace(name))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePasskey(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM passkeys WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) PasskeyByCredentialID(ctx context.Context, credentialID []byte) (*Passkey, error) {
	var p Passkey
	err := s.Pool.QueryRow(ctx, `SELECT id, user_id, name, credential_id, credential, created_at, last_used_at FROM passkeys WHERE credential_id=$1`, credentialID).
		Scan(&p.ID, &p.UserID, &p.Name, &p.CredentialID, &p.Credential, &p.CreatedAt, &p.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) TouchPasskey(ctx context.Context, id uuid.UUID, credential json.RawMessage) error {
	_, err := s.Pool.Exec(ctx, `UPDATE passkeys SET credential=$2, last_used_at=NOW() WHERE id=$1`, id, credential)
	return err
}

func (s *Store) SaveWebAuthnSession(ctx context.Context, userID *uuid.UUID, kind string, session json.RawMessage, ttl time.Duration) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO webauthn_sessions (user_id, kind, session, expires_at)
		VALUES ($1,$2,$3,$4) RETURNING id
	`, userID, kind, session, time.Now().Add(ttl)).Scan(&id)
	return id, err
}

func (s *Store) TakeWebAuthnSession(ctx context.Context, id uuid.UUID, kind string) (json.RawMessage, *uuid.UUID, error) {
	var raw json.RawMessage
	var userID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		DELETE FROM webauthn_sessions
		WHERE id=$1 AND kind=$2 AND expires_at>NOW()
		RETURNING session, user_id
	`, id, kind).Scan(&raw, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	return raw, userID, err
}

func (s *Store) SetHostingerVM(ctx context.Context, teamID, serverID uuid.UUID, vmID string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE servers SET hostinger_vm_id=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2`, serverID, teamID, vmID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

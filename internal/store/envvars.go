package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/envvars"
	"github.com/jackc/pgx/v5"
)

type EnvVar struct {
	ID           uuid.UUID `json:"id"`
	TeamID       uuid.UUID `json:"team_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	Key          string    `json:"key"`
	Value        string    `json:"value,omitempty"`
	IsPreview    bool      `json:"is_preview"`
	IsRuntime    bool      `json:"is_runtime"`
	IsBuildtime  bool      `json:"is_buildtime"`
	IsLiteral    bool      `json:"is_literal"`
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type SharedEnvVar struct {
	ID        uuid.UUID  `json:"id"`
	TeamID    uuid.UUID  `json:"team_id"`
	ScopeType string     `json:"scope_type"`
	ScopeID   *uuid.UUID `json:"scope_id"`
	Key       string     `json:"key"`
	Value     string     `json:"value,omitempty"`
	IsLiteral bool       `json:"is_literal"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Store) ListEnvVars(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, reveal bool) ([]EnvVar, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, key, value_enc, is_preview, is_runtime, is_buildtime, is_literal, comment, created_at
		FROM environment_variables
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3
		ORDER BY sort_order, key
	`, teamID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EnvVar
	for rows.Next() {
		var v EnvVar
		var enc string
		if err := rows.Scan(&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Key, &enc,
			&v.IsPreview, &v.IsRuntime, &v.IsBuildtime, &v.IsLiteral, &v.Comment, &v.CreatedAt); err != nil {
			return nil, err
		}
		if reveal {
			plain, err := s.Box.DecryptString(enc)
			if err != nil {
				return nil, err
			}
			v.Value = plain
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) UpsertEnvVar(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, key, value string, runtime, buildtime, literal bool, comment string) (*EnvVar, error) {
	enc, err := s.Box.EncryptString(value)
	if err != nil {
		return nil, err
	}
	var v EnvVar
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO environment_variables (team_id, resource_type, resource_id, key, value_enc, is_runtime, is_buildtime, is_literal, comment)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (resource_type, resource_id, key, is_preview) DO UPDATE
		SET value_enc=EXCLUDED.value_enc, is_runtime=EXCLUDED.is_runtime, is_buildtime=EXCLUDED.is_buildtime,
		    is_literal=EXCLUDED.is_literal, comment=EXCLUDED.comment, updated_at=NOW()
		RETURNING id, team_id, resource_type, resource_id, key, is_preview, is_runtime, is_buildtime, is_literal, comment, created_at
	`, teamID, resourceType, resourceID, key, enc, runtime, buildtime, literal, comment).Scan(
		&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Key, &v.IsPreview, &v.IsRuntime, &v.IsBuildtime, &v.IsLiteral, &v.Comment, &v.CreatedAt,
	)
	v.Value = value
	return &v, err
}

func (s *Store) DeleteEnvVar(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM environment_variables WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ResolvedEnvMap(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, projectID, envID, serverID *uuid.UUID) (map[string]string, error) {
	vars, err := s.ListEnvVars(ctx, teamID, resourceType, resourceID, true)
	if err != nil {
		return nil, err
	}
	scopes := map[string]map[string]string{
		"team":        {},
		"project":     {},
		"environment": {},
		"server":      {},
	}
	loadShared := func(scopeType string, scopeID *uuid.UUID) error {
		q := `SELECT key, value_enc FROM shared_environment_variables WHERE team_id=$1 AND scope_type=$2`
		args := []any{teamID, scopeType}
		if scopeID == nil {
			q += ` AND scope_id IS NULL`
		} else {
			q += ` AND scope_id=$3`
			args = append(args, *scopeID)
		}
		rows, err := s.Pool.Query(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var key, enc string
			if err := rows.Scan(&key, &enc); err != nil {
				return err
			}
			plain, err := s.Box.DecryptString(enc)
			if err != nil {
				return err
			}
			scopes[scopeType][key] = plain
		}
		return rows.Err()
	}
	_ = loadShared("team", nil)
	if projectID != nil {
		_ = loadShared("project", projectID)
	}
	if envID != nil {
		_ = loadShared("environment", envID)
	}
	if serverID != nil {
		_ = loadShared("server", serverID)
	}

	out := map[string]string{}
	for _, v := range vars {
		if !v.IsRuntime {
			continue
		}
		if v.IsLiteral {
			out[v.Key] = v.Value
			continue
		}
		out[v.Key] = envvars.Resolve(v.Value, scopes)
	}
	return out, nil
}

func (s *Store) UpsertSharedEnv(ctx context.Context, teamID uuid.UUID, scopeType string, scopeID *uuid.UUID, key, value string, literal bool) (*SharedEnvVar, error) {
	enc, err := s.Box.EncryptString(value)
	if err != nil {
		return nil, err
	}
	var v SharedEnvVar
	// Unique index uses COALESCE(scope_id, zero-uuid). Prefer update-then-insert for reliable upsert.
	err = s.Pool.QueryRow(ctx, `
		UPDATE shared_environment_variables
		SET value_enc=$5, is_literal=$6, updated_at=NOW()
		WHERE team_id=$1 AND scope_type=$2 AND key=$4
		  AND COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'::uuid)
		    = COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
		RETURNING id, team_id, scope_type, scope_id, key, is_literal, created_at
	`, teamID, scopeType, scopeID, key, enc, literal).Scan(
		&v.ID, &v.TeamID, &v.ScopeType, &v.ScopeID, &v.Key, &v.IsLiteral, &v.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.Pool.QueryRow(ctx, `
			INSERT INTO shared_environment_variables (team_id, scope_type, scope_id, key, value_enc, is_literal)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, team_id, scope_type, scope_id, key, is_literal, created_at
		`, teamID, scopeType, scopeID, key, enc, literal).Scan(
			&v.ID, &v.TeamID, &v.ScopeType, &v.ScopeID, &v.Key, &v.IsLiteral, &v.CreatedAt,
		)
		// Concurrent insert race: fall back to update.
		if err != nil && (strings.Contains(err.Error(), "shared_env_unique") || strings.Contains(err.Error(), "duplicate")) {
			err = s.Pool.QueryRow(ctx, `
				UPDATE shared_environment_variables
				SET value_enc=$5, is_literal=$6, updated_at=NOW()
				WHERE team_id=$1 AND scope_type=$2 AND key=$4
				  AND COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'::uuid)
				    = COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
				RETURNING id, team_id, scope_type, scope_id, key, is_literal, created_at
			`, teamID, scopeType, scopeID, key, enc, literal).Scan(
				&v.ID, &v.TeamID, &v.ScopeType, &v.ScopeID, &v.Key, &v.IsLiteral, &v.CreatedAt,
			)
		}
	}
	if err != nil {
		return nil, err
	}
	v.Value = value
	return &v, nil
}

func (s *Store) ListSharedEnv(ctx context.Context, teamID uuid.UUID, scopeType string, scopeID *uuid.UUID, reveal bool) ([]SharedEnvVar, error) {
	q := `SELECT id, team_id, scope_type, scope_id, key, value_enc, is_literal, created_at FROM shared_environment_variables WHERE team_id=$1 AND scope_type=$2`
	args := []any{teamID, scopeType}
	if scopeID == nil {
		q += ` AND scope_id IS NULL`
	} else {
		q += ` AND scope_id=$3`
		args = append(args, *scopeID)
	}
	q += ` ORDER BY key`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SharedEnvVar
	for rows.Next() {
		var v SharedEnvVar
		var enc string
		if err := rows.Scan(&v.ID, &v.TeamID, &v.ScopeType, &v.ScopeID, &v.Key, &enc, &v.IsLiteral, &v.CreatedAt); err != nil {
			return nil, err
		}
		if reveal {
			plain, err := s.Box.DecryptString(enc)
			if err != nil {
				return nil, err
			}
			v.Value = plain
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetWebhookSecret(ctx context.Context, appID uuid.UUID) (string, error) {
	var enc string
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(webhook_secret_enc,'') FROM applications WHERE id=$1`, appID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil || enc == "" {
		return "", err
	}
	return s.Box.DecryptString(enc)
}

func (s *Store) SetWebhookSecret(ctx context.Context, teamID, appID uuid.UUID, secret string) error {
	enc, err := s.Box.EncryptString(secret)
	if err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE applications SET webhook_secret_enc=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2`, appID, teamID, enc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CancelDeployment(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE deployments SET status='cancelled', finished_at=NOW(), updated_at=NOW(), error_message='cancelled by user'
		WHERE id=$1 AND team_id=$2 AND status IN ('queued','in_progress')
	`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetEnvironment(ctx context.Context, teamID, id uuid.UUID) (*Environment, error) {
	var e Environment
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, project_id, name, description, created_at FROM environments WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&e.ID, &e.TeamID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	empty, err := s.EnvironmentIsEmpty(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	e.IsEmpty = empty
	return &e, nil
}

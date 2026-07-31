package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CloudProviderToken struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Provider    string    `json:"provider"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CloudInitScript struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Script    string    `json:"script,omitempty"`
}

func (s *Store) ListCloudProviderTokens(ctx context.Context, teamID uuid.UUID) ([]CloudProviderToken, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, provider, name, description, created_at, updated_at
		FROM cloud_provider_tokens WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CloudProviderToken
	for rows.Next() {
		var t CloudProviderToken
		if err := rows.Scan(&t.ID, &t.TeamID, &t.Provider, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetCloudProviderToken(ctx context.Context, teamID, id uuid.UUID) (*CloudProviderToken, error) {
	var t CloudProviderToken
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, provider, name, description, created_at, updated_at
		FROM cloud_provider_tokens WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&t.ID, &t.TeamID, &t.Provider, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

func (s *Store) GetCloudProviderTokenMaterial(ctx context.Context, teamID, id uuid.UUID) (tokenEnc string, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT token_enc FROM cloud_provider_tokens WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&tokenEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return tokenEnc, err
}

func (s *Store) CreateCloudProviderToken(ctx context.Context, teamID uuid.UUID, provider, name, description, tokenEnc string) (*CloudProviderToken, error) {
	var t CloudProviderToken
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO cloud_provider_tokens (team_id, provider, name, description, token_enc)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, team_id, provider, name, description, created_at, updated_at
	`, teamID, provider, name, description, tokenEnc).Scan(
		&t.ID, &t.TeamID, &t.Provider, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &t, nil
}

func (s *Store) UpdateCloudProviderToken(ctx context.Context, teamID, id uuid.UUID, name, description string, tokenEnc *string) (*CloudProviderToken, error) {
	var t CloudProviderToken
	var err error
	if tokenEnc != nil {
		err = s.Pool.QueryRow(ctx, `
			UPDATE cloud_provider_tokens
			SET name=$3, description=$4, token_enc=$5, updated_at=NOW()
			WHERE id=$1 AND team_id=$2
			RETURNING id, team_id, provider, name, description, created_at, updated_at
		`, id, teamID, name, description, *tokenEnc).Scan(
			&t.ID, &t.TeamID, &t.Provider, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt,
		)
	} else {
		err = s.Pool.QueryRow(ctx, `
			UPDATE cloud_provider_tokens
			SET name=$3, description=$4, updated_at=NOW()
			WHERE id=$1 AND team_id=$2
			RETURNING id, team_id, provider, name, description, created_at, updated_at
		`, id, teamID, name, description).Scan(
			&t.ID, &t.TeamID, &t.Provider, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &t, nil
}

func (s *Store) DeleteCloudProviderToken(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM cloud_provider_tokens WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListCloudInitScripts(ctx context.Context, teamID uuid.UUID) ([]CloudInitScript, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, name, created_at, updated_at
		FROM cloud_init_scripts WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CloudInitScript
	for rows.Next() {
		var sc CloudInitScript
		if err := rows.Scan(&sc.ID, &sc.TeamID, &sc.Name, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) GetCloudInitScript(ctx context.Context, teamID, id uuid.UUID) (*CloudInitScript, string, error) {
	var sc CloudInitScript
	var enc string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, name, script_enc, created_at, updated_at
		FROM cloud_init_scripts WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&sc.ID, &sc.TeamID, &sc.Name, &enc, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &sc, enc, err
}

func (s *Store) CreateCloudInitScript(ctx context.Context, teamID uuid.UUID, name, scriptEnc string) (*CloudInitScript, error) {
	var sc CloudInitScript
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO cloud_init_scripts (team_id, name, script_enc)
		VALUES ($1,$2,$3)
		RETURNING id, team_id, name, created_at, updated_at
	`, teamID, name, scriptEnc).Scan(&sc.ID, &sc.TeamID, &sc.Name, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &sc, nil
}

func (s *Store) UpdateCloudInitScript(ctx context.Context, teamID, id uuid.UUID, name, scriptEnc string) (*CloudInitScript, error) {
	var sc CloudInitScript
	err := s.Pool.QueryRow(ctx, `
		UPDATE cloud_init_scripts
		SET name=$3, script_enc=$4, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
		RETURNING id, team_id, name, created_at, updated_at
	`, id, teamID, name, scriptEnc).Scan(&sc.ID, &sc.TeamID, &sc.Name, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &sc, nil
}

func (s *Store) DeleteCloudInitScript(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM cloud_init_scripts WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

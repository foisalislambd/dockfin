package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// IntegrationToken is a team secret-manager credential. The token itself is never serialized.
type IntegrationToken struct {
	ID        uuid.UUID       `json:"id"`
	TeamID    uuid.UUID       `json:"team_id"`
	Provider  string          `json:"provider"`
	Name      string          `json:"name"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SecretManagerLink binds one resource to one integration token.
// Settings hold source coordinates only (project, path). Secret values are not stored.
type SecretManagerLink struct {
	ID                 uuid.UUID       `json:"id"`
	TeamID             uuid.UUID       `json:"team_id"`
	ResourceType       string          `json:"resource_type"`
	ResourceID         uuid.UUID       `json:"resource_id"`
	IntegrationTokenID uuid.UUID       `json:"integration_token_id"`
	Provider           string          `json:"provider"`
	TokenName          string          `json:"token_name"`
	Settings           json.RawMessage `json:"settings"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// SecretManagerMaterial is the decrypted material needed to fetch remote secrets.
// Token must not be written to logs or API responses.
type SecretManagerMaterial struct {
	Provider string
	Token    string
	Metadata json.RawMessage
	Settings json.RawMessage
}

func (s *Store) ListIntegrationTokens(ctx context.Context, teamID uuid.UUID) ([]IntegrationToken, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, provider, name, metadata, created_at, updated_at
		FROM integration_tokens WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IntegrationToken
	for rows.Next() {
		tok, err := scanIntegrationToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	return out, rows.Err()
}

func (s *Store) GetIntegrationToken(ctx context.Context, teamID, id uuid.UUID) (*IntegrationToken, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, provider, name, metadata, created_at, updated_at
		FROM integration_tokens WHERE id=$1 AND team_id=$2
	`, id, teamID)
	tok, err := scanIntegrationToken(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

func (s *Store) IntegrationTokenSecret(ctx context.Context, teamID, id uuid.UUID) (provider, token string, metadata json.RawMessage, err error) {
	var enc string
	err = s.Pool.QueryRow(ctx, `
		SELECT provider, token_enc, metadata FROM integration_tokens WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&provider, &enc, &metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, ErrNotFound
	}
	if err != nil {
		return "", "", nil, err
	}
	token, err = s.Box.DecryptString(enc)
	if err != nil {
		return "", "", nil, err
	}
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	return provider, token, metadata, nil
}

func (s *Store) CreateIntegrationToken(ctx context.Context, teamID uuid.UUID, provider, name, tokenEnc string, metadata json.RawMessage) (*IntegrationToken, error) {
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO integration_tokens (team_id, provider, name, token_enc, metadata)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, team_id, provider, name, metadata, created_at, updated_at
	`, teamID, provider, name, tokenEnc, metadata)
	tok, err := scanIntegrationToken(row)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &tok, nil
}

func (s *Store) UpdateIntegrationToken(ctx context.Context, teamID, id uuid.UUID, name string, tokenEnc *string, metadata json.RawMessage) (*IntegrationToken, error) {
	var row pgx.Row
	if tokenEnc != nil {
		row = s.Pool.QueryRow(ctx, `
			UPDATE integration_tokens
			SET name=$3, token_enc=$4, metadata=$5, updated_at=NOW()
			WHERE id=$1 AND team_id=$2
			RETURNING id, team_id, provider, name, metadata, created_at, updated_at
		`, id, teamID, name, *tokenEnc, metadata)
	} else {
		row = s.Pool.QueryRow(ctx, `
			UPDATE integration_tokens
			SET name=$3, metadata=$4, updated_at=NOW()
			WHERE id=$1 AND team_id=$2
			RETURNING id, team_id, provider, name, metadata, created_at, updated_at
		`, id, teamID, name, metadata)
	}
	tok, err := scanIntegrationToken(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &tok, nil
}

func (s *Store) DeleteIntegrationToken(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM integration_tokens WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetSecretManagerLink(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) (*SecretManagerLink, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT l.id, l.team_id, l.resource_type, l.resource_id, l.integration_token_id,
		       t.provider, t.name, l.settings, l.created_at, l.updated_at
		FROM secret_manager_links l
		JOIN integration_tokens t ON t.id = l.integration_token_id AND t.team_id = l.team_id
		WHERE l.team_id=$1 AND l.resource_type=$2 AND l.resource_id=$3
	`, teamID, resourceType, resourceID)
	link, err := scanSecretManagerLink(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (s *Store) UpsertSecretManagerLink(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID, tokenID uuid.UUID, settings json.RawMessage) (*SecretManagerLink, error) {
	if len(settings) == 0 {
		settings = []byte(`{}`)
	}
	var tokTeam uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT team_id FROM integration_tokens WHERE id=$1`, tokenID).Scan(&tokTeam)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && tokTeam != teamID) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO secret_manager_links (team_id, resource_type, resource_id, integration_token_id, settings)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (resource_type, resource_id) DO UPDATE
		SET integration_token_id=EXCLUDED.integration_token_id, settings=EXCLUDED.settings, updated_at=NOW()
		WHERE secret_manager_links.team_id = EXCLUDED.team_id
		RETURNING id
	`, teamID, resourceType, resourceID, tokenID, settings)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return s.GetSecretManagerLink(ctx, teamID, resourceType, resourceID)
}

func (s *Store) DeleteSecretManagerLink(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM secret_manager_links WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3
	`, teamID, resourceType, resourceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SecretManagerMaterial decrypts the token linked to a resource.
func (s *Store) SecretManagerMaterial(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) (*SecretManagerMaterial, error) {
	var enc string
	mat := &SecretManagerMaterial{}
	err := s.Pool.QueryRow(ctx, `
		SELECT t.provider, t.token_enc, t.metadata, l.settings
		FROM secret_manager_links l
		JOIN integration_tokens t ON t.id = l.integration_token_id AND t.team_id = l.team_id
		WHERE l.team_id=$1 AND l.resource_type=$2 AND l.resource_id=$3
	`, teamID, resourceType, resourceID).Scan(&mat.Provider, &enc, &mat.Metadata, &mat.Settings)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	mat.Token, err = s.Box.DecryptString(enc)
	if err != nil {
		return nil, err
	}
	return mat, nil
}

type tokenScanner interface {
	Scan(dest ...any) error
}

func scanIntegrationToken(row tokenScanner) (IntegrationToken, error) {
	var tok IntegrationToken
	err := row.Scan(&tok.ID, &tok.TeamID, &tok.Provider, &tok.Name, &tok.Metadata, &tok.CreatedAt, &tok.UpdatedAt)
	if err != nil {
		return tok, err
	}
	if len(tok.Metadata) == 0 {
		tok.Metadata = []byte(`{}`)
	}
	return tok, nil
}

func scanSecretManagerLink(row tokenScanner) (SecretManagerLink, error) {
	var link SecretManagerLink
	err := row.Scan(&link.ID, &link.TeamID, &link.ResourceType, &link.ResourceID, &link.IntegrationTokenID,
		&link.Provider, &link.TokenName, &link.Settings, &link.CreatedAt, &link.UpdatedAt)
	if err != nil {
		return link, err
	}
	if len(link.Settings) == 0 {
		link.Settings = []byte(`{}`)
	}
	return link, nil
}

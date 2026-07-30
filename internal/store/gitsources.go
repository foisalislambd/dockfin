package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GitSource struct {
	ID             uuid.UUID `json:"id"`
	TeamID         uuid.UUID `json:"team_id"`
	Provider       string    `json:"provider"`
	Name           string    `json:"name"`
	AppID          string    `json:"app_id"`
	InstallationID string    `json:"installation_id,omitempty"`
	ClientID       string    `json:"client_id,omitempty"`
	HTMLURL        string    `json:"html_url"`
	APIURL         string    `json:"api_url"`
	IsPublic       bool      `json:"is_public"`
	CreatedAt      time.Time `json:"created_at"`
}

type GitSourceSecrets struct {
	AppID            string
	InstallationID   string
	ClientID         string
	ClientSecretEnc  string
	PrivateKeyEnc    string
	WebhookSecretEnc string
	Name             string
	HTMLURL          string
	APIURL           string
}

func (s *Store) CreateGitSource(ctx context.Context, teamID uuid.UUID, provider, name, appID, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc, htmlURL, apiURL string) (*GitSource, error) {
	if provider == "" {
		provider = "github"
	}
	var gs GitSource
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO git_sources (team_id, provider, name, app_id, client_id, client_secret_enc, private_key_enc, webhook_secret_enc, html_url, api_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, team_id, provider, name, app_id, installation_id, client_id, html_url, api_url, is_public, created_at
	`, teamID, provider, name, appID, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc, htmlURL, apiURL).Scan(
		&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.AppID, &gs.InstallationID, &gs.ClientID, &gs.HTMLURL, &gs.APIURL, &gs.IsPublic, &gs.CreatedAt,
	)
	return &gs, err
}

func (s *Store) ListGitSources(ctx context.Context, teamID uuid.UUID) ([]GitSource, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, provider, name, app_id, installation_id, client_id, html_url, api_url, is_public, created_at
		FROM git_sources WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitSource
	for rows.Next() {
		var gs GitSource
		if err := rows.Scan(&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.AppID, &gs.InstallationID, &gs.ClientID, &gs.HTMLURL, &gs.APIURL, &gs.IsPublic, &gs.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, gs)
	}
	return out, rows.Err()
}

func (s *Store) GetGitSource(ctx context.Context, teamID, id uuid.UUID) (*GitSource, error) {
	var gs GitSource
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, provider, name, app_id, installation_id, client_id, html_url, api_url, is_public, created_at
		FROM git_sources WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.AppID, &gs.InstallationID, &gs.ClientID, &gs.HTMLURL, &gs.APIURL, &gs.IsPublic, &gs.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &gs, err
}

func (s *Store) UpdateGitSourceInstallation(ctx context.Context, teamID, id uuid.UUID, installationID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE git_sources SET installation_id=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2
	`, id, teamID, installationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteGitSource(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM git_sources WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetGitSourceSecrets(ctx context.Context, teamID, id uuid.UUID) (*GitSourceSecrets, error) {
	var sec GitSourceSecrets
	err := s.Pool.QueryRow(ctx, `
		SELECT app_id, installation_id, client_id, client_secret_enc, private_key_enc, webhook_secret_enc, name, html_url, api_url
		FROM git_sources WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&sec.AppID, &sec.InstallationID, &sec.ClientID, &sec.ClientSecretEnc, &sec.PrivateKeyEnc, &sec.WebhookSecretEnc, &sec.Name, &sec.HTMLURL, &sec.APIURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &sec, err
}

func (s *Store) SaveGitSetupState(ctx context.Context, state string, teamID, sourceID uuid.UUID, expires time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO git_source_setup_states (state, team_id, source_id, expires_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (state) DO UPDATE SET team_id=EXCLUDED.team_id, source_id=EXCLUDED.source_id, expires_at=EXCLUDED.expires_at
	`, state, teamID, sourceID, expires)
	return err
}

func (s *Store) ConsumeGitSetupState(ctx context.Context, state string) (teamID, sourceID uuid.UUID, err error) {
	err = s.Pool.QueryRow(ctx, `
		DELETE FROM git_source_setup_states
		WHERE state=$1 AND expires_at > NOW()
		RETURNING team_id, source_id
	`, state).Scan(&teamID, &sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	return teamID, sourceID, err
}

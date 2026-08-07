package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GitSource struct {
	ID             uuid.UUID `json:"id"`
	TeamID         uuid.UUID `json:"team_id"`
	Provider       string    `json:"provider"`
	Name           string    `json:"name"`
	Organization   string    `json:"organization,omitempty"`
	AppID          string    `json:"app_id"`
	InstallationID string    `json:"installation_id,omitempty"`
	ClientID       string    `json:"client_id,omitempty"`
	HTMLURL        string    `json:"html_url"`
	APIURL         string    `json:"api_url"`
	CustomUser     string    `json:"custom_user"`
	CustomPort     int       `json:"custom_port"`
	IsPublic       bool      `json:"is_public"`
	IsSystemWide   bool      `json:"is_system_wide"`
	HasPrivateKey  bool      `json:"has_private_key"`
	Configured     bool      `json:"configured"` // app_id + private key present
	Installed      bool      `json:"installed"`  // installation_id present
	Applications   int       `json:"applications_count"`
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
	Organization     string
	CustomUser       string
	CustomPort       int
}

type GitSourceAppRef struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	ProjectID     uuid.UUID `json:"project_id"`
	ProjectName   string    `json:"project_name"`
	EnvName       string    `json:"environment_name"`
	BuildPack     string    `json:"build_pack"`
}

const gitSourceSelect = `
	id, team_id, provider, name, COALESCE(organization,''), app_id, installation_id, client_id,
	html_url, api_url, COALESCE(custom_user,'git'), COALESCE(custom_port,22), is_public,
	COALESCE(is_system_wide,FALSE), (private_key_enc <> ''), created_at`

func scanGitSource(scan func(dest ...any) error) (*GitSource, error) {
	var gs GitSource
	err := scan(
		&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.Organization, &gs.AppID, &gs.InstallationID, &gs.ClientID,
		&gs.HTMLURL, &gs.APIURL, &gs.CustomUser, &gs.CustomPort, &gs.IsPublic, &gs.IsSystemWide, &gs.HasPrivateKey, &gs.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	gs.Configured = gs.AppID != "" && gs.AppID != "0" && gs.HasPrivateKey
	gs.Installed = gs.InstallationID != "" && gs.InstallationID != "0"
	return &gs, nil
}

func (s *Store) CreateGitSource(ctx context.Context, teamID uuid.UUID, provider, name, organization, appID, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc, htmlURL, apiURL, customUser string, customPort int) (*GitSource, error) {
	if provider == "" {
		provider = "github"
	}
	if htmlURL == "" {
		htmlURL = "https://github.com"
	}
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	if customUser == "" {
		customUser = "git"
	}
	if customPort == 0 {
		customPort = 22
	}
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO git_sources (
			team_id, provider, name, organization, app_id, client_id, client_secret_enc, private_key_enc,
			webhook_secret_enc, html_url, api_url, custom_user, custom_port
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id
	`, teamID, provider, name, organization, appID, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc, htmlURL, apiURL, customUser, customPort).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetGitSource(ctx, teamID, id)
}

func (s *Store) ListGitSources(ctx context.Context, teamID uuid.UUID) ([]GitSource, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+gitSourceSelect+`,
			(SELECT COUNT(*)::int FROM applications a WHERE a.git_source_id = git_sources.id)
		FROM git_sources
		WHERE team_id=$1 AND is_public=FALSE
		ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitSource
	for rows.Next() {
		var gs GitSource
		var count int
		if err := rows.Scan(
			&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.Organization, &gs.AppID, &gs.InstallationID, &gs.ClientID,
			&gs.HTMLURL, &gs.APIURL, &gs.CustomUser, &gs.CustomPort, &gs.IsPublic, &gs.IsSystemWide, &gs.HasPrivateKey, &gs.CreatedAt,
			&count,
		); err != nil {
			return nil, err
		}
		gs.Configured = gs.AppID != "" && gs.AppID != "0" && gs.HasPrivateKey
		gs.Installed = gs.InstallationID != "" && gs.InstallationID != "0"
		gs.Applications = count
		out = append(out, gs)
	}
	return out, rows.Err()
}

func (s *Store) GetGitSource(ctx context.Context, teamID, id uuid.UUID) (*GitSource, error) {
	var gs GitSource
	var count int
	err := s.Pool.QueryRow(ctx, `
		SELECT `+gitSourceSelect+`,
			(SELECT COUNT(*)::int FROM applications a WHERE a.git_source_id = git_sources.id)
		FROM git_sources WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.Organization, &gs.AppID, &gs.InstallationID, &gs.ClientID,
		&gs.HTMLURL, &gs.APIURL, &gs.CustomUser, &gs.CustomPort, &gs.IsPublic, &gs.IsSystemWide, &gs.HasPrivateKey, &gs.CreatedAt,
		&count,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	gs.Configured = gs.AppID != "" && gs.AppID != "0" && gs.HasPrivateKey
	gs.Installed = gs.InstallationID != "" && gs.InstallationID != "0"
	gs.Applications = count
	return &gs, nil
}

type UpdateGitSourceInput struct {
	Name             *string
	Organization     *string
	AppID            *string
	InstallationID   *string
	ClientID         *string
	ClientSecretEnc  *string // empty string pointer means keep; non-empty replace
	PrivateKeyEnc    *string
	WebhookSecretEnc *string
	HTMLURL          *string
	APIURL           *string
	CustomUser       *string
	CustomPort       *int
	IsSystemWide     *bool
}

func (s *Store) UpdateGitSource(ctx context.Context, teamID, id uuid.UUID, in UpdateGitSourceInput) (*GitSource, error) {
	cur, err := s.GetGitSourceSecrets(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	gs, err := s.GetGitSource(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	name, org, appID, instID, clientID := gs.Name, gs.Organization, gs.AppID, gs.InstallationID, gs.ClientID
	htmlURL, apiURL, customUser, customPort := gs.HTMLURL, gs.APIURL, gs.CustomUser, gs.CustomPort
	isSystemWide := gs.IsSystemWide
	clientSecretEnc, privateKeyEnc, webhookSecretEnc := cur.ClientSecretEnc, cur.PrivateKeyEnc, cur.WebhookSecretEnc

	if in.Name != nil {
		name = *in.Name
	}
	if in.Organization != nil {
		org = *in.Organization
	}
	if in.AppID != nil {
		appID = *in.AppID
	}
	if in.InstallationID != nil {
		instID = *in.InstallationID
	}
	if in.ClientID != nil {
		clientID = *in.ClientID
	}
	if in.ClientSecretEnc != nil && *in.ClientSecretEnc != "" {
		clientSecretEnc = *in.ClientSecretEnc
	}
	if in.PrivateKeyEnc != nil && *in.PrivateKeyEnc != "" {
		privateKeyEnc = *in.PrivateKeyEnc
	}
	if in.WebhookSecretEnc != nil && *in.WebhookSecretEnc != "" {
		webhookSecretEnc = *in.WebhookSecretEnc
	}
	if in.HTMLURL != nil {
		htmlURL = *in.HTMLURL
	}
	if in.APIURL != nil {
		apiURL = *in.APIURL
	}
	if in.CustomUser != nil {
		customUser = *in.CustomUser
	}
	if in.CustomPort != nil {
		customPort = *in.CustomPort
	}
	if in.IsSystemWide != nil {
		isSystemWide = *in.IsSystemWide
	}

	tag, err := s.Pool.Exec(ctx, `
		UPDATE git_sources SET
			name=$3, organization=$4, app_id=$5, installation_id=$6, client_id=$7,
			client_secret_enc=$8, private_key_enc=$9, webhook_secret_enc=$10,
			html_url=$11, api_url=$12, custom_user=$13, custom_port=$14, is_system_wide=$15,
			updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, id, teamID, name, org, appID, instID, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc, htmlURL, apiURL, customUser, customPort, isSystemWide)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetGitSource(ctx, teamID, id)
}

func (s *Store) GetGitSourceByID(ctx context.Context, id uuid.UUID) (*GitSource, error) {
	var gs GitSource
	var count int
	err := s.Pool.QueryRow(ctx, `
		SELECT `+gitSourceSelect+`,
			(SELECT COUNT(*)::int FROM applications a WHERE a.git_source_id = git_sources.id)
		FROM git_sources WHERE id=$1
	`, id).Scan(
		&gs.ID, &gs.TeamID, &gs.Provider, &gs.Name, &gs.Organization, &gs.AppID, &gs.InstallationID, &gs.ClientID,
		&gs.HTMLURL, &gs.APIURL, &gs.CustomUser, &gs.CustomPort, &gs.IsPublic, &gs.IsSystemWide, &gs.HasPrivateKey, &gs.CreatedAt,
		&count,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	gs.Configured = gs.AppID != "" && gs.AppID != "0" && gs.HasPrivateKey
	gs.Installed = gs.InstallationID != "" && gs.InstallationID != "0"
	gs.Applications = count
	return &gs, nil
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

func (s *Store) ApplyManifestCredentials(ctx context.Context, teamID, id uuid.UUID, appID, slug, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE git_sources SET
			app_id=$3, name=COALESCE(NULLIF($4,''), name), client_id=$5,
			client_secret_enc=$6, private_key_enc=$7, webhook_secret_enc=$8,
			updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, id, teamID, appID, slug, clientID, clientSecretEnc, privateKeyEnc, webhookSecretEnc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountAppsUsingGitSource(ctx context.Context, teamID, id uuid.UUID) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM applications WHERE team_id=$1 AND git_source_id=$2
	`, teamID, id).Scan(&n)
	return n, err
}

func (s *Store) ListAppsUsingGitSource(ctx context.Context, teamID, id uuid.UUID) ([]GitSourceAppRef, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.name, a.environment_id, e.project_id, p.name, e.name, a.build_pack
		FROM applications a
		JOIN environments e ON e.id = a.environment_id
		JOIN projects p ON p.id = e.project_id
		WHERE a.team_id=$1 AND a.git_source_id=$2
		ORDER BY p.name, e.name, a.name
	`, teamID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitSourceAppRef
	for rows.Next() {
		var r GitSourceAppRef
		if err := rows.Scan(&r.ID, &r.Name, &r.EnvironmentID, &r.ProjectID, &r.ProjectName, &r.EnvName, &r.BuildPack); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteGitSource(ctx context.Context, teamID, id uuid.UUID) error {
	n, err := s.CountAppsUsingGitSource(ctx, teamID, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict
	}
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
		SELECT app_id, installation_id, client_id, client_secret_enc, private_key_enc, webhook_secret_enc,
			name, html_url, api_url, COALESCE(organization,''), COALESCE(custom_user,'git'), COALESCE(custom_port,22)
		FROM git_sources WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&sec.AppID, &sec.InstallationID, &sec.ClientID, &sec.ClientSecretEnc, &sec.PrivateKeyEnc, &sec.WebhookSecretEnc,
		&sec.Name, &sec.HTMLURL, &sec.APIURL, &sec.Organization, &sec.CustomUser, &sec.CustomPort,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &sec, err
}

// GetGitSourceByAppID finds a configured GitHub App source by numeric/string app_id
// (X-GitHub-Hook-Installation-Target-Id). Returns source metadata and encrypted secrets.
func (s *Store) GetGitSourceByAppID(ctx context.Context, appID string) (*GitSource, *GitSourceSecrets, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, nil, ErrNotFound
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT `+gitSourceSelect+`,
			(SELECT COUNT(*)::int FROM applications a WHERE a.git_source_id = git_sources.id),
			client_secret_enc, private_key_enc, webhook_secret_enc
		FROM git_sources
		WHERE app_id=$1 AND is_public=FALSE
		ORDER BY updated_at DESC NULLS LAST, created_at DESC
		LIMIT 1
	`, appID)
	var appsCount int
	var clientSecretEnc, privateKeyEnc, webhookSecretEnc string
	gs, err := scanGitSource(func(dest ...any) error {
		args := append(dest, &appsCount, &clientSecretEnc, &privateKeyEnc, &webhookSecretEnc)
		return row.Scan(args...)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	gs.Applications = appsCount
	sec := &GitSourceSecrets{
		AppID:            gs.AppID,
		InstallationID:   gs.InstallationID,
		ClientID:         gs.ClientID,
		ClientSecretEnc:  clientSecretEnc,
		PrivateKeyEnc:    privateKeyEnc,
		WebhookSecretEnc: webhookSecretEnc,
		Name:             gs.Name,
		HTMLURL:          gs.HTMLURL,
		APIURL:           gs.APIURL,
		Organization:     gs.Organization,
		CustomUser:       gs.CustomUser,
		CustomPort:       gs.CustomPort,
	}
	return gs, sec, nil
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

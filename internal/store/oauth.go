package store

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OauthMaterial is the decrypted config needed to drive an OAuth2 flow for a provider.
type OauthMaterial struct {
	Provider     string
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Tenant       string
	BaseURL      string
}

// OauthProviderPublic is the safe-to-expose shape returned by the public providers listing.
type OauthProviderPublic struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

var oauthProviderDisplayNames = map[string]string{
	"github":     "GitHub",
	"gitlab":     "GitLab",
	"bitbucket":  "Bitbucket",
	"google":     "Google",
	"azure":      "Microsoft Azure",
	"discord":    "Discord",
	"authentik":  "Authentik",
	"clerk":      "Clerk",
	"infomaniak": "Infomaniak",
	"zitadel":    "Zitadel",
}

// OauthProviderDisplayName returns a human-friendly label for a provider slug.
func OauthProviderDisplayName(provider string) string {
	if name, ok := oauthProviderDisplayNames[strings.ToLower(strings.TrimSpace(provider))]; ok {
		return name
	}
	return provider
}

// FindOauthAccountUser looks up the user linked to a given provider account.
func (s *Store) FindOauthAccountUser(ctx context.Context, provider, providerUserID string) (*User, error) {
	var userID uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT user_id FROM oauth_accounts WHERE provider=$1 AND provider_user_id=$2
	`, provider, providerUserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, userID)
}

// LinkOauthAccount attaches (or refreshes) a provider account link to an existing user.
func (s *Store) LinkOauthAccount(ctx context.Context, userID uuid.UUID, provider, providerUserID, email string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO oauth_accounts (user_id, provider, provider_user_id, email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_user_id) DO UPDATE SET email = EXCLUDED.email
	`, userID, provider, providerUserID, strings.ToLower(strings.TrimSpace(email)))
	return err
}

// CreateUserOAuth creates a new user with an empty password hash (OAuth-only account)
// plus a personal team, mirroring CreateUserWithPersonalTeam.
func (s *Store) CreateUserOAuth(ctx context.Context, email, name string) (*User, *Team, error) {
	if strings.TrimSpace(name) == "" {
		name = email
	}
	return s.CreateUserWithPersonalTeam(ctx, email, name, "")
}

// GetOauthSettingMaterial returns the decrypted settings needed to drive an OAuth2 flow.
func (s *Store) GetOauthSettingMaterial(ctx context.Context, provider string) (*OauthMaterial, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	var (
		m         OauthMaterial
		secretEnc string
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT provider, enabled, client_id, client_secret_enc, redirect_uri, tenant, base_url
		FROM oauth_settings WHERE provider = $1
	`, provider).Scan(&m.Provider, &m.Enabled, &m.ClientID, &secretEnc, &m.RedirectURI, &m.Tenant, &m.BaseURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if secretEnc != "" && s.Box != nil {
		if plain, err := s.Box.DecryptString(secretEnc); err == nil {
			m.ClientSecret = plain
		}
	}
	return &m, nil
}

// ListEnabledOauthProviders returns the public-safe list of providers enabled for login.
func (s *Store) ListEnabledOauthProviders(ctx context.Context) ([]OauthProviderPublic, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT provider FROM oauth_settings WHERE enabled = TRUE ORDER BY provider
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OauthProviderPublic{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, OauthProviderPublic{Provider: p, Name: OauthProviderDisplayName(p)})
	}
	return out, rows.Err()
}

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ApiToken struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	TeamID      uuid.UUID  `json:"team_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Abilities   []string   `json:"abilities"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type TeamMember struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type TeamInvitation struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Token     string    `json:"token,omitempty"`
	InvitedBy uuid.UUID `json:"invited_by"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTeam creates a new non-personal team owned by ownerUserID.
func (s *Store) CreateTeam(ctx context.Context, ownerUserID uuid.UUID, name, description string) (*Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name required", ErrConflict)
	}
	description = strings.TrimSpace(description)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var t Team
	err = tx.QueryRow(ctx, `
		INSERT INTO teams (name, description, personal) VALUES ($1, $2, FALSE)
		RETURNING id, name, description, personal, created_at
	`, name, description).Scan(&t.ID, &t.Name, &t.Description, &t.Personal, &t.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'owner')
	`, t.ID, ownerUserID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	t.Role = "owner"
	return &t, nil
}

func (s *Store) ListApiTokens(ctx context.Context, userID, teamID uuid.UUID) ([]ApiToken, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, team_id, name, token_prefix, abilities, last_used_at, expires_at, created_at
		FROM api_tokens
		WHERE user_id=$1 AND team_id=$2
		ORDER BY created_at DESC
	`, userID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ApiToken
	for rows.Next() {
		var t ApiToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.TeamID, &t.Name, &t.TokenPrefix, &t.Abilities, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateApiToken(ctx context.Context, userID, teamID uuid.UUID, name, plainToken string, abilities []string, expiresAt *time.Time) (*ApiToken, error) {
	if name == "" {
		return nil, errors.New("name required")
	}
	if len(abilities) == 0 {
		abilities = []string{"*"}
	}
	prefix := plainToken
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	var t ApiToken
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO api_tokens (user_id, team_id, name, token_hash, token_prefix, abilities, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, user_id, team_id, name, token_prefix, abilities, last_used_at, expires_at, created_at
	`, userID, teamID, name, HashToken(plainToken), prefix, abilities, expiresAt).Scan(
		&t.ID, &t.UserID, &t.TeamID, &t.Name, &t.TokenPrefix, &t.Abilities, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt,
	)
	return &t, err
}

func (s *Store) DeleteApiToken(ctx context.Context, userID, teamID, tokenID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM api_tokens WHERE id=$1 AND user_id=$2 AND team_id=$3
	`, tokenID, userID, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetApiTokenByPlain(ctx context.Context, plain string) (*ApiToken, *User, error) {
	var t ApiToken
	err := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, team_id, name, token_prefix, abilities, last_used_at, expires_at, created_at
		FROM api_tokens
		WHERE token_hash=$1 AND (expires_at IS NULL OR expires_at > NOW())
	`, HashToken(plain)).Scan(
		&t.ID, &t.UserID, &t.TeamID, &t.Name, &t.TokenPrefix, &t.Abilities, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE api_tokens SET last_used_at=NOW() WHERE id=$1`, t.ID)
	u, err := s.GetUserByID(ctx, t.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &t, u, nil
}

func (s *Store) ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]TeamMember, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT u.id, u.email, u.name, tm.role, tm.created_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id=$1
		ORDER BY tm.role, u.email
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
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
		var owners int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM team_members WHERE team_id=$1 AND role='owner' FOR UPDATE`, teamID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return errors.New("cannot remove the last owner")
		}
	}
	tag, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id=$1 AND user_id=$2`, teamID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateInvitation(ctx context.Context, teamID, invitedBy uuid.UUID, email, role, token string, expiresAt time.Time) (*TeamInvitation, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("email required")
	}
	if role != "admin" && role != "member" {
		role = "member"
	}
	var inv TeamInvitation
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO team_invitations (team_id, email, role, token, invited_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, team_id, email, role, token, invited_by, expires_at, created_at
	`, teamID, email, role, token, invitedBy, expiresAt).Scan(
		&inv.ID, &inv.TeamID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedBy, &inv.ExpiresAt, &inv.CreatedAt,
	)
	return &inv, err
}

func (s *Store) ListInvitations(ctx context.Context, teamID uuid.UUID) ([]TeamInvitation, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, email, role, invited_by, expires_at, created_at
		FROM team_invitations
		WHERE team_id=$1 AND expires_at > NOW()
		ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamInvitation
	for rows.Next() {
		var inv TeamInvitation
		if err := rows.Scan(&inv.ID, &inv.TeamID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) DeleteInvitation(ctx context.Context, teamID, inviteID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM team_invitations WHERE id=$1 AND team_id=$2`, inviteID, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetInvitationByToken(ctx context.Context, token string) (*TeamInvitation, error) {
	var inv TeamInvitation
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, email, role, token, invited_by, expires_at, created_at
		FROM team_invitations
		WHERE token=$1 AND expires_at > NOW()
	`, token).Scan(
		&inv.ID, &inv.TeamID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedBy, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &inv, err
}

type InvitationPreview struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TeamName  string    `json:"team_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PreviewInvitation returns invite metadata without consuming the token.
// Safe for crawlers / link unfurls (GET must never accept).
func (s *Store) PreviewInvitation(ctx context.Context, token string) (*InvitationPreview, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrNotFound
	}
	var p InvitationPreview
	err := s.Pool.QueryRow(ctx, `
		SELECT i.email, i.role, t.name, i.expires_at
		FROM team_invitations i
		JOIN teams t ON t.id = i.team_id
		WHERE i.token=$1 AND i.expires_at > NOW()
	`, token).Scan(&p.Email, &p.Role, &p.TeamName, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID, userEmail string) (*Team, error) {
	inv, err := s.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(inv.Email, userEmail) {
		return nil, ErrUnauthorized
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES ($1,$2,$3)
		ON CONFLICT (team_id, user_id) DO NOTHING
	`, inv.TeamID, userID, inv.Role)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `DELETE FROM team_invitations WHERE id=$1`, inv.ID)
	if err != nil {
		return nil, err
	}
	var t Team
	err = tx.QueryRow(ctx, `
		SELECT t.id, t.name, t.description, t.personal, t.created_at, tm.role
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = $2
		WHERE t.id=$1
	`, inv.TeamID, userID).Scan(&t.ID, &t.Name, &t.Description, &t.Personal, &t.CreatedAt, &t.Role)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &t, nil
}

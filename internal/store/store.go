package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrUnauthorized = errors.New("unauthorized")

type Store struct {
	Pool *pgxpool.Pool
	Box  *crypto.Box
}

func New(pool *pgxpool.Pool, box *crypto.Box) *Store {
	return &Store{Pool: pool, Box: box}
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type User struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

type Team struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Personal    bool      `json:"personal"`
	Role        string    `json:"role,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	CurrentTeamID *uuid.UUID
	ExpiresAt     time.Time
}

func (s *Store) CreateUserWithPersonalTeam(ctx context.Context, email, name, passwordHash string) (*User, *Team, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, email_verified, created_at
	`, strings.ToLower(strings.TrimSpace(email)), name, passwordHash).Scan(
		&u.ID, &u.Email, &u.Name, &u.EmailVerified, &u.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_lower_idx") || strings.Contains(err.Error(), "duplicate") {
			return nil, nil, ErrConflict
		}
		return nil, nil, err
	}

	var t Team
	teamName := name + "'s Team"
	err = tx.QueryRow(ctx, `
		INSERT INTO teams (name, personal) VALUES ($1, TRUE)
		RETURNING id, name, description, personal, created_at
	`, teamName).Scan(&t.ID, &t.Name, &t.Description, &t.Personal, &t.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'owner')
	`, t.ID, u.ID)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	t.Role = "owner"
	return &u, &t, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	var u User
	var hash string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, name, email_verified, created_at, password_hash
		FROM users WHERE LOWER(email) = LOWER($1)
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.EmailVerified, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, name, email_verified, created_at FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.EmailVerified, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *Store) CreateSession(ctx context.Context, userID, teamID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, current_team_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, HashToken(token), teamID, expiresAt)
	return err
}

func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	var sess Session
	err := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, current_team_id, expires_at
		FROM sessions WHERE token_hash = $1 AND expires_at > NOW()
	`, HashToken(token)).Scan(&sess.ID, &sess.UserID, &sess.CurrentTeamID, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = NOW() WHERE id = $1`, sess.ID)
	return &sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, HashToken(token))
	return err
}

func (s *Store) SetCurrentTeam(ctx context.Context, sessionID, teamID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sessions SET current_team_id = $1 WHERE id = $2`, teamID, sessionID)
	return err
}

func (s *Store) ListTeamsForUser(ctx context.Context, userID uuid.UUID) ([]Team, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT t.id, t.name, t.description, t.personal, t.created_at, tm.role
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1
		ORDER BY t.personal DESC, t.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Personal, &t.CreatedAt, &t.Role); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UserRoleOnTeam(ctx context.Context, userID, teamID uuid.UUID) (string, error) {
	var role string
	err := s.Pool.QueryRow(ctx, `
		SELECT role FROM team_members WHERE user_id = $1 AND team_id = $2
	`, userID, teamID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthorized
	}
	return role, err
}

func (s *Store) RegistrationEnabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.Pool.QueryRow(ctx, `SELECT is_registration_enabled FROM instance_settings WHERE id = 1`).Scan(&enabled)
	return enabled, err
}

func (s *Store) EnsureMembership(ctx context.Context, userID, teamID uuid.UUID) error {
	_, err := s.UserRoleOnTeam(ctx, userID, teamID)
	return err
}

func MustUUID() uuid.UUID { return uuid.New() }

func ParseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid: %w", err)
	}
	return id, nil
}

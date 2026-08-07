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

// AuthChallenge is a short-lived record used to complete a second factor (e.g. TOTP) login step.
type AuthChallenge struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Kind      string    `json:"kind"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// UserHasTOTP reports whether TOTP 2FA is enabled for a user.
func (s *Store) UserHasTOTP(ctx context.Context, userID uuid.UUID) (bool, error) {
	var enabled bool
	err := s.Pool.QueryRow(ctx, `SELECT totp_enabled FROM users WHERE id = $1`, userID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return enabled, err
}

// GetUserTOTPSecret returns the decrypted TOTP secret (may be empty if never set up) and whether it's enabled.
func (s *Store) GetUserTOTPSecret(ctx context.Context, userID uuid.UUID) (secret string, enabled bool, err error) {
	var secretEnc string
	err = s.Pool.QueryRow(ctx, `
		SELECT totp_secret_enc, totp_enabled FROM users WHERE id = $1
	`, userID).Scan(&secretEnc, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrNotFound
	}
	if err != nil {
		return "", false, err
	}
	if secretEnc == "" {
		return "", enabled, nil
	}
	if s.Box == nil {
		return "", enabled, fmt.Errorf("encryption box not configured")
	}
	secret, err = s.Box.DecryptString(secretEnc)
	return secret, enabled, err
}

// SetUserTOTPSecret stores a pending (not-yet-enabled) TOTP secret for a user, encrypted.
// Refuses to overwrite when 2FA is already enabled — caller must disable first.
func (s *Store) SetUserTOTPSecret(ctx context.Context, userID uuid.UUID, secretPlain string) error {
	if s.Box == nil {
		return fmt.Errorf("encryption box not configured")
	}
	enc, err := s.Box.EncryptString(secretPlain)
	if err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE users SET totp_secret_enc = $1, totp_enabled = FALSE
		WHERE id = $2 AND totp_enabled = FALSE
	`, enc, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		enabled, err := s.UserHasTOTP(ctx, userID)
		if err != nil {
			return err
		}
		if enabled {
			return fmt.Errorf("%w: 2fa already enabled; disable before re-setup", ErrConflict)
		}
		return ErrNotFound
	}
	return nil
}

// EnableUserTOTP marks TOTP as enabled and stores the (plaintext) recovery codes encrypted.
func (s *Store) EnableUserTOTP(ctx context.Context, userID uuid.UUID, recoveryCodes []string) error {
	if s.Box == nil {
		return fmt.Errorf("encryption box not configured")
	}
	enc, err := s.Box.EncryptString(strings.Join(recoveryCodes, ","))
	if err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE users SET totp_enabled = TRUE, totp_recovery_codes_enc = $1 WHERE id = $2
	`, enc, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DisableUserTOTP clears TOTP configuration for a user.
func (s *Store) DisableUserTOTP(ctx context.Context, userID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE users SET totp_enabled = FALSE, totp_secret_enc = '', totp_recovery_codes_enc = '' WHERE id = $1
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumeRecoveryCode checks a one-time recovery code and, if valid, removes it from the stored set.
func (s *Store) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return false, nil
	}
	var enc string
	err := s.Pool.QueryRow(ctx, `SELECT totp_recovery_codes_enc FROM users WHERE id = $1`, userID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if enc == "" || s.Box == nil {
		return false, nil
	}
	plain, err := s.Box.DecryptString(enc)
	if err != nil {
		return false, err
	}
	var codes []string
	for _, c := range strings.Split(plain, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			codes = append(codes, c)
		}
	}
	found := -1
	for i, c := range codes {
		if strings.EqualFold(c, code) {
			found = i
			break
		}
	}
	if found == -1 {
		return false, nil
	}
	codes = append(codes[:found], codes[found+1:]...)
	newEnc, err := s.Box.EncryptString(strings.Join(codes, ","))
	if err != nil {
		return false, err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE users SET totp_recovery_codes_enc = $1 WHERE id = $2`, newEnc, userID)
	return true, err
}

// CreateAuthChallenge inserts a new short-lived 2FA challenge for a user.
func (s *Store) CreateAuthChallenge(ctx context.Context, userID uuid.UUID, kind string, ttl time.Duration) (*AuthChallenge, error) {
	if kind == "" {
		kind = "totp"
	}
	var c AuthChallenge
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO auth_challenges (user_id, kind, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, kind, expires_at, created_at
	`, userID, kind, time.Now().UTC().Add(ttl)).Scan(&c.ID, &c.UserID, &c.Kind, &c.ExpiresAt, &c.CreatedAt)
	return &c, err
}

// GetAuthChallenge fetches a non-expired challenge without consuming it.
func (s *Store) GetAuthChallenge(ctx context.Context, id uuid.UUID) (*AuthChallenge, error) {
	var c AuthChallenge
	err := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, kind, expires_at, created_at FROM auth_challenges
		WHERE id = $1 AND expires_at > NOW()
	`, id).Scan(&c.ID, &c.UserID, &c.Kind, &c.ExpiresAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// ConsumeAuthChallenge deletes and returns a non-expired challenge (one-time use).
func (s *Store) ConsumeAuthChallenge(ctx context.Context, id uuid.UUID) (*AuthChallenge, error) {
	var c AuthChallenge
	err := s.Pool.QueryRow(ctx, `
		DELETE FROM auth_challenges WHERE id = $1 AND expires_at > NOW()
		RETURNING id, user_id, kind, expires_at, created_at
	`, id).Scan(&c.ID, &c.UserID, &c.Kind, &c.ExpiresAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// CreatePasswordResetToken records a hashed, single-use password reset token for an email.
func (s *Store) CreatePasswordResetToken(ctx context.Context, email, tokenPlain string, ttl time.Duration) error {
	email = strings.ToLower(strings.TrimSpace(email))
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (email, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, email, HashToken(tokenPlain), time.Now().UTC().Add(ttl))
	return err
}

// ConsumePasswordResetToken validates and deletes a reset token, returning the associated email.
func (s *Store) ConsumePasswordResetToken(ctx context.Context, tokenPlain string) (string, error) {
	var email string
	err := s.Pool.QueryRow(ctx, `
		DELETE FROM password_reset_tokens WHERE token_hash = $1 AND expires_at > NOW()
		RETURNING email
	`, HashToken(tokenPlain)).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return email, err
}

// UpdatePassword sets a new password hash for a user (e.g. after password reset).
func (s *Store) UpdatePassword(ctx context.Context, userID uuid.UUID, newHash string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2
	`, newHash, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

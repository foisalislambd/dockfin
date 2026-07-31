package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type NotificationSetting struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	Channel   string    `json:"channel"`
	Enabled   bool      `json:"enabled"`
	ConfigEnc string    `json:"-"`
	Events    []string  `json:"events"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) ListNotificationSettings(ctx context.Context, teamID uuid.UUID) ([]NotificationSetting, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, channel, enabled, config_enc, events, created_at, updated_at
		FROM notification_settings WHERE team_id=$1 ORDER BY channel
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotificationSetting
	for rows.Next() {
		var n NotificationSetting
		if err := rows.Scan(&n.ID, &n.TeamID, &n.Channel, &n.Enabled, &n.ConfigEnc, &n.Events, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNotificationSetting(ctx context.Context, teamID uuid.UUID, channel string) (*NotificationSetting, error) {
	var n NotificationSetting
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, channel, enabled, config_enc, events, created_at, updated_at
		FROM notification_settings WHERE team_id=$1 AND channel=$2
	`, teamID, channel).Scan(
		&n.ID, &n.TeamID, &n.Channel, &n.Enabled, &n.ConfigEnc, &n.Events, &n.CreatedAt, &n.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &n, err
}

func (s *Store) UpsertNotificationSetting(ctx context.Context, teamID uuid.UUID, channel string, enabled bool, configEnc string, events []string) (*NotificationSetting, error) {
	var n NotificationSetting
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO notification_settings (team_id, channel, enabled, config_enc, events)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (team_id, channel) DO UPDATE
		SET enabled=$3, config_enc=$4, events=$5, updated_at=NOW()
		RETURNING id, team_id, channel, enabled, config_enc, events, created_at, updated_at
	`, teamID, channel, enabled, configEnc, events).Scan(
		&n.ID, &n.TeamID, &n.Channel, &n.Enabled, &n.ConfigEnc, &n.Events, &n.CreatedAt, &n.UpdatedAt,
	)
	return &n, err
}

func (s *Store) ListEnabledNotifications(ctx context.Context, teamID uuid.UUID) ([]NotificationSetting, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, channel, enabled, config_enc, events, created_at, updated_at
		FROM notification_settings WHERE team_id=$1 AND enabled=TRUE
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotificationSetting
	for rows.Next() {
		var n NotificationSetting
		if err := rows.Scan(&n.ID, &n.TeamID, &n.Channel, &n.Enabled, &n.ConfigEnc, &n.Events, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) TeamMemberEmails(ctx context.Context, teamID uuid.UUID) ([]string, error) {
	members, err := s.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(members))
	for _, m := range members {
		if m.Email != "" {
			out = append(out, m.Email)
		}
	}
	return out, nil
}

// SMTPMaterial returns decrypted SMTP credentials from instance settings for sending mail.
func (s *Store) SMTPMaterial(ctx context.Context) (user, pass, resendKey string, err error) {
	var userEnc, passEnc, resendEnc string
	err = s.Pool.QueryRow(ctx, `
		SELECT smtp_username_enc, smtp_password_enc, resend_api_key_enc FROM instance_settings WHERE id=1
	`).Scan(&userEnc, &passEnc, &resendEnc)
	if err != nil {
		return "", "", "", err
	}
	if userEnc != "" && s.Box != nil {
		user, _ = s.Box.DecryptString(userEnc)
	}
	if passEnc != "" && s.Box != nil {
		pass, _ = s.Box.DecryptString(passEnc)
	}
	if resendEnc != "" && s.Box != nil {
		resendKey, _ = s.Box.DecryptString(resendEnc)
	}
	return user, pass, resendKey, nil
}

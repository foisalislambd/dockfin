package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DatabaseInitScript is one saved init file. The list is encrypted together.
type DatabaseInitScript struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (s *Store) DatabaseInitScripts(ctx context.Context, teamID, id uuid.UUID) ([]DatabaseInitScript, error) {
	var enc string
	err := s.Pool.QueryRow(ctx, `
		SELECT init_scripts_enc FROM databases WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if enc == "" {
		return nil, nil
	}
	plain, err := s.Box.DecryptString(enc)
	if err != nil {
		return nil, err
	}
	var out []DatabaseInitScript
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) SetDatabaseInitScripts(ctx context.Context, teamID, id uuid.UUID, scripts []DatabaseInitScript) error {
	enc := ""
	if len(scripts) > 0 {
		raw, err := json.Marshal(scripts)
		if err != nil {
			return err
		}
		enc, err = s.Box.EncryptString(string(raw))
		if err != nil {
			return err
		}
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE databases SET init_scripts_enc=$3 WHERE id=$1 AND team_id=$2
	`, id, teamID, enc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CopyDatabaseInitScripts(ctx context.Context, teamID, fromID, toID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE databases AS dst
		SET init_scripts_enc = src.init_scripts_enc
		FROM databases AS src
		WHERE dst.team_id=$1 AND dst.id=$3 AND src.team_id=$1 AND src.id=$2
	`, teamID, fromID, toID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

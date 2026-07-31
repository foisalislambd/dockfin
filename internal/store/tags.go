package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Tag struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListTags(ctx context.Context, teamID uuid.UUID) ([]Tag, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, name, color, created_at FROM tags WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.TeamID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []Tag{}
	}
	return out, rows.Err()
}

func (s *Store) CreateTag(ctx context.Context, teamID uuid.UUID, name, color string) (*Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name required")
	}
	if color == "" {
		color = "#14b8a6"
	}
	var t Tag
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO tags (team_id, name, color) VALUES ($1,$2,$3)
		ON CONFLICT (team_id, name) DO UPDATE SET color=EXCLUDED.color
		RETURNING id, team_id, name, color, created_at
	`, teamID, name, color).Scan(&t.ID, &t.TeamID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) DeleteTag(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM tags WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListResourceTags(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]Tag, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT t.id, t.team_id, t.name, t.color, t.created_at
		FROM tags t
		JOIN taggables tg ON tg.tag_id = t.id
		WHERE t.team_id=$1 AND tg.resource_type=$2 AND tg.resource_id=$3
		ORDER BY t.name
	`, teamID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.TeamID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []Tag{}
	}
	return out, rows.Err()
}

// ListEnvironmentResourceTags returns tags keyed by "type:id" for all resources in an environment.
func (s *Store) ListEnvironmentResourceTags(ctx context.Context, teamID, envID uuid.UUID) (map[string][]Tag, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT tg.resource_type, tg.resource_id, t.id, t.team_id, t.name, t.color, t.created_at
		FROM taggables tg
		JOIN tags t ON t.id = tg.tag_id
		WHERE t.team_id=$1 AND (
			(tg.resource_type='application' AND tg.resource_id IN (SELECT id FROM applications WHERE environment_id=$2 AND team_id=$1))
			OR (tg.resource_type='database' AND tg.resource_id IN (SELECT id FROM databases WHERE environment_id=$2 AND team_id=$1))
			OR (tg.resource_type='service' AND tg.resource_id IN (SELECT id FROM services WHERE environment_id=$2 AND team_id=$1 AND deleted_at IS NULL))
		)
		ORDER BY t.name
	`, teamID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Tag{}
	for rows.Next() {
		var rt string
		var rid uuid.UUID
		var t Tag
		if err := rows.Scan(&rt, &rid, &t.ID, &t.TeamID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		key := rt + ":" + rid.String()
		out[key] = append(out[key], t)
	}
	return out, rows.Err()
}

func (s *Store) AttachTag(ctx context.Context, teamID, tagID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	switch resourceType {
	case "application", "database", "service":
	default:
		return ErrNotFound
	}
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM tags WHERE id=$1 AND team_id=$2`, tagID, teamID).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	// Ensure the target resource belongs to this team (never attach to arbitrary UUIDs).
	switch resourceType {
	case "application":
		if _, err := s.GetApplication(ctx, teamID, resourceID); err != nil {
			return err
		}
	case "database":
		if _, err := s.GetDatabase(ctx, teamID, resourceID); err != nil {
			return err
		}
	case "service":
		if _, err := s.GetService(ctx, teamID, resourceID); err != nil {
			return err
		}
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO taggables (tag_id, resource_type, resource_id) VALUES ($1,$2,$3)
		ON CONFLICT DO NOTHING
	`, tagID, resourceType, resourceID)
	return err
}

func (s *Store) DetachTag(ctx context.Context, teamID, tagID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM taggables tg
		USING tags t
		WHERE tg.tag_id=t.id AND t.team_id=$1 AND tg.tag_id=$2 AND tg.resource_type=$3 AND tg.resource_id=$4
	`, teamID, tagID, resourceType, resourceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

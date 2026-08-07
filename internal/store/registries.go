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

type DockerRegistry struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Username    string    `json:"username"`
	HasPassword bool      `json:"has_password"`
	PasswordEnc string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Store) CreateDockerRegistry(ctx context.Context, teamID uuid.UUID, name, url, username, passwordEnc string) (*DockerRegistry, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, errors.New("name required")
	}
	if url == "" {
		url = "docker.io"
	}
	var r DockerRegistry
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO docker_registries (team_id, name, url, username, password_enc)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, team_id, name, url, username, password_enc, created_at
	`, teamID, name, url, strings.TrimSpace(username), passwordEnc).Scan(
		&r.ID, &r.TeamID, &r.Name, &r.URL, &r.Username, &r.PasswordEnc, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.HasPassword = strings.TrimSpace(r.PasswordEnc) != ""
	return &r, nil
}

func (s *Store) ListDockerRegistries(ctx context.Context, teamID uuid.UUID) ([]DockerRegistry, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, name, url, username, password_enc, created_at
		FROM docker_registries WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DockerRegistry
	for rows.Next() {
		var r DockerRegistry
		if err := rows.Scan(&r.ID, &r.TeamID, &r.Name, &r.URL, &r.Username, &r.PasswordEnc, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.HasPassword = strings.TrimSpace(r.PasswordEnc) != ""
		r.PasswordEnc = ""
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetDockerRegistry(ctx context.Context, teamID, id uuid.UUID) (*DockerRegistry, error) {
	var r DockerRegistry
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, name, url, username, password_enc, created_at
		FROM docker_registries WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&r.ID, &r.TeamID, &r.Name, &r.URL, &r.Username, &r.PasswordEnc, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.HasPassword = strings.TrimSpace(r.PasswordEnc) != ""
	return &r, nil
}

func (s *Store) UpdateDockerRegistry(ctx context.Context, teamID, id uuid.UUID, name, url, username string, passwordEnc *string) (*DockerRegistry, error) {
	cur, err := s.GetDockerRegistry(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) != "" {
		cur.Name = strings.TrimSpace(name)
	}
	if strings.TrimSpace(url) != "" {
		cur.URL = strings.TrimSpace(url)
	}
	cur.Username = strings.TrimSpace(username)
	enc := cur.PasswordEnc
	if passwordEnc != nil && strings.TrimSpace(*passwordEnc) != "" {
		enc = *passwordEnc
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE docker_registries SET name=$3, url=$4, username=$5, password_enc=$6, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, id, teamID, cur.Name, cur.URL, cur.Username, enc)
	if err != nil {
		return nil, err
	}
	return s.GetDockerRegistry(ctx, teamID, id)
}

func (s *Store) DeleteDockerRegistry(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM docker_registries WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListAdditionalDestinations(ctx context.Context, teamID, appID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT ad.destination_id
		FROM additional_destinations ad
		JOIN applications a ON a.id = ad.application_id
		WHERE ad.application_id=$1 AND a.team_id=$2
		ORDER BY ad.destination_id
	`, appID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) SetAdditionalDestinations(ctx context.Context, teamID, appID uuid.UUID, destinationIDs []uuid.UUID) error {
	app, err := s.GetApplication(ctx, teamID, appID)
	if err != nil {
		return err
	}
	var primaryServer uuid.UUID
	if app.DestinationID != nil {
		if primary, err := s.GetDestination(ctx, teamID, *app.DestinationID); err == nil {
			primaryServer = primary.ServerID
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM additional_destinations WHERE application_id=$1`, appID); err != nil {
		return err
	}
	seen := map[uuid.UUID]bool{}
	for _, destID := range destinationIDs {
		if seen[destID] {
			continue
		}
		seen[destID] = true
		if app.DestinationID != nil && *app.DestinationID == destID {
			continue
		}
		dest, err := s.GetDestination(ctx, teamID, destID)
		if err != nil {
			return err
		}
		// Same container/compose project name would clobber the primary deploy.
		if primaryServer != uuid.Nil && dest.ServerID == primaryServer {
			return fmt.Errorf("%w: additional destination %q is on the same server as the primary — pick another server", ErrConflict, dest.Name)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO additional_destinations (application_id, destination_id) VALUES ($1,$2)
		`, appID, destID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

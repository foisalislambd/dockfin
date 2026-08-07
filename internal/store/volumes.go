package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Volume struct {
	ID           uuid.UUID `json:"id"`
	TeamID       uuid.UUID `json:"team_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	Name         string    `json:"name"`
	MountPath    string    `json:"mount_path"`
	HostPath     string    `json:"host_path"`
	IsFile       bool      `json:"is_file"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) ListVolumes(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]Volume, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, name, mount_path, COALESCE(host_path,''), is_file, created_at
		FROM volumes WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3
		ORDER BY name
	`, teamID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Volume
	for rows.Next() {
		var v Volume
		if err := rows.Scan(&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) UpsertVolume(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, name, mountPath, hostPath string, isFile bool) (*Volume, error) {
	name = strings.TrimSpace(name)
	mountPath = strings.TrimSpace(mountPath)
	hostPath = strings.TrimSpace(hostPath)
	if name == "" || mountPath == "" {
		return nil, errors.New("name and mount_path required")
	}
	if !strings.HasPrefix(mountPath, "/") {
		mountPath = "/" + mountPath
	}
	var existing Volume
	err := s.Pool.QueryRow(ctx, `
		SELECT id FROM volumes
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3 AND name=$4
	`, teamID, resourceType, resourceID, name).Scan(&existing.ID)
	if err == nil {
		var v Volume
		err = s.Pool.QueryRow(ctx, `
			UPDATE volumes SET mount_path=$2, host_path=$3, is_file=$4
			WHERE id=$1
			RETURNING id, team_id, resource_type, resource_id, name, mount_path, COALESCE(host_path,''), is_file, created_at
		`, existing.ID, mountPath, hostPath, isFile).Scan(
			&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.CreatedAt,
		)
		return &v, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var v Volume
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO volumes (team_id, resource_type, resource_id, name, mount_path, host_path, is_file)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, team_id, resource_type, resource_id, name, mount_path, COALESCE(host_path,''), is_file, created_at
	`, teamID, resourceType, resourceID, name, mountPath, hostPath, isFile).Scan(
		&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) DeleteVolume(ctx context.Context, teamID, volumeID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM volumes WHERE id=$1 AND team_id=$2`, volumeID, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetVolume(ctx context.Context, teamID, volumeID uuid.UUID) (*Volume, error) {
	var v Volume
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, resource_type, resource_id, name, mount_path, COALESCE(host_path,''), is_file, created_at
		FROM volumes WHERE id=$1 AND team_id=$2
	`, volumeID, teamID).Scan(
		&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &v, err
}

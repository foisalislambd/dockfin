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
	HasContent   bool      `json:"has_content"`
	CreatedAt    time.Time `json:"created_at"`
}

const volumeSelect = `
	id, team_id, resource_type, resource_id, name, mount_path, COALESCE(host_path,''), is_file,
	(content_enc <> ''), created_at
`

func scanVolume(row interface{ Scan(...any) error }, v *Volume) error {
	return row.Scan(
		&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile,
		&v.HasContent, &v.CreatedAt,
	)
}

func (s *Store) ListVolumes(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]Volume, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+volumeSelect+`
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
		if err := scanVolume(rows, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UpsertVolume saves a directory or file mount. A nil content leaves stored file content unchanged.
// An empty content clears it. Directory mounts never keep content.
func (s *Store) UpsertVolume(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, name, mountPath, hostPath string, isFile bool, content *string) (*Volume, error) {
	name = strings.TrimSpace(name)
	mountPath = strings.TrimSpace(mountPath)
	hostPath = strings.TrimSpace(hostPath)
	if name == "" || mountPath == "" {
		return nil, errors.New("name and mount_path required")
	}
	if !strings.HasPrefix(mountPath, "/") {
		mountPath = "/" + mountPath
	}
	enc := ""
	setContent := content != nil || !isFile
	if isFile && content != nil && *content != "" {
		var err error
		enc, err = s.Box.EncryptString(*content)
		if err != nil {
			return nil, err
		}
	}
	var existing uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT id FROM volumes
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3 AND name=$4
	`, teamID, resourceType, resourceID, name).Scan(&existing)
	if err == nil {
		var v Volume
		err = s.Pool.QueryRow(ctx, `
			UPDATE volumes SET mount_path=$2, host_path=$3, is_file=$4,
				content_enc = CASE WHEN $5 THEN $6 ELSE content_enc END
			WHERE id=$1
			RETURNING `+volumeSelect+`
		`, existing, mountPath, hostPath, isFile, setContent, enc).Scan(
			&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.HasContent, &v.CreatedAt,
		)
		return &v, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var v Volume
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO volumes (team_id, resource_type, resource_id, name, mount_path, host_path, is_file, content_enc)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+volumeSelect+`
	`, teamID, resourceType, resourceID, name, mountPath, hostPath, isFile, enc).Scan(
		&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.HasContent, &v.CreatedAt,
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
		SELECT `+volumeSelect+` FROM volumes WHERE id=$1 AND team_id=$2
	`, volumeID, teamID).Scan(
		&v.ID, &v.TeamID, &v.ResourceType, &v.ResourceID, &v.Name, &v.MountPath, &v.HostPath, &v.IsFile, &v.HasContent, &v.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &v, err
}

// VolumeContent returns the decrypted file body. Directory mounts and empty files return an empty string.
func (s *Store) VolumeContent(ctx context.Context, teamID, volumeID uuid.UUID) (string, error) {
	var enc string
	var isFile bool
	err := s.Pool.QueryRow(ctx, `
		SELECT is_file, content_enc FROM volumes WHERE id=$1 AND team_id=$2
	`, volumeID, teamID).Scan(&isFile, &enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !isFile || enc == "" {
		return "", nil
	}
	return s.Box.DecryptString(enc)
}

// CloneVolumes copies mounts onto another resource. Application host paths are cleared so the
// copy does not share the source directory. Database paths inside Dockfin's data tree are cleared too.
func (s *Store) CloneVolumes(ctx context.Context, teamID uuid.UUID, resourceType string, fromID, toID uuid.UUID) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT name, mount_path, COALESCE(host_path,''), is_file, content_enc
		FROM volumes WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3
	`, teamID, resourceType, fromID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		name, mount, host, enc string
		isFile                 bool
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.mount, &r.host, &r.isFile, &r.enc); err != nil {
			return err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range pending {
		host := r.host
		if resourceType == "application" || strings.Contains(host, fromID.String()) {
			host = ""
		}
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO volumes (team_id, resource_type, resource_id, name, mount_path, host_path, is_file, content_enc)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, teamID, resourceType, toID, r.name, r.mount, host, r.isFile, r.enc); err != nil {
			return err
		}
	}
	return nil
}

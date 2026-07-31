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

// InstanceBackupResourceID is the fixed synthetic resource id for instance DB backups.
var InstanceBackupResourceID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

const InstanceBackupResourceType = "instance"

// InstanceBackupConfig is Coolify-like local backup settings for the Goolify DB.
type InstanceBackupConfig struct {
	Configured  bool   `json:"configured"`
	Enabled     bool   `json:"enabled"`
	Frequency   string `json:"frequency"`
	Retention   int    `json:"retention"`
	Description string `json:"description"`
	DBUser      string `json:"db_user"`
	DBName      string `json:"db_name"`
	Container   string `json:"container"`
	Name        string `json:"name"`
	UUID        string `json:"uuid"`
}

func (s *Store) GetInstanceBackupConfig(ctx context.Context) (*InstanceBackupConfig, error) {
	var c InstanceBackupConfig
	err := s.Pool.QueryRow(ctx, `
		SELECT backup_configured, backup_enabled, backup_frequency, backup_retention,
			backup_description, backup_db_user, backup_db_name, backup_container
		FROM instance_settings WHERE id = 1
	`).Scan(
		&c.Configured, &c.Enabled, &c.Frequency, &c.Retention,
		&c.Description, &c.DBUser, &c.DBName, &c.Container,
	)
	if err != nil {
		return nil, err
	}
	c.Name = "goolify-db"
	c.UUID = InstanceBackupResourceID.String()
	return &c, nil
}

type InstanceBackupPatch struct {
	Enabled     *bool   `json:"enabled"`
	Frequency   *string `json:"frequency"`
	Retention   *int    `json:"retention"`
	Description *string `json:"description"`
	Container   *string `json:"container"`
	DBUser      *string `json:"db_user"`
	DBName      *string `json:"db_name"`
}

func (s *Store) ConfigureInstanceBackup(ctx context.Context, container, dbUser, dbName, description string) (*InstanceBackupConfig, error) {
	if strings.TrimSpace(dbUser) == "" {
		dbUser = "goolify"
	}
	if strings.TrimSpace(dbName) == "" {
		dbName = "goolify"
	}
	if strings.TrimSpace(description) == "" {
		description = "Goolify database"
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE instance_settings SET
			backup_configured=TRUE,
			backup_enabled=TRUE,
			backup_frequency=COALESCE(NULLIF(backup_frequency,''), '0 0 * * *'),
			backup_retention=CASE WHEN backup_retention > 0 THEN backup_retention ELSE 7 END,
			backup_description=$1,
			backup_db_user=$2,
			backup_db_name=$3,
			backup_container=$4,
			updated_at=NOW()
		WHERE id = 1
	`, description, dbUser, dbName, strings.TrimSpace(container))
	if err != nil {
		return nil, err
	}
	return s.GetInstanceBackupConfig(ctx)
}

func (s *Store) UpdateInstanceBackupConfig(ctx context.Context, patch InstanceBackupPatch) (*InstanceBackupConfig, error) {
	cur, err := s.GetInstanceBackupConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cur.Configured {
		return nil, fmt.Errorf("%w: instance backup is not configured yet", ErrConflict)
	}
	if patch.Enabled != nil {
		cur.Enabled = *patch.Enabled
	}
	if patch.Frequency != nil {
		f := strings.TrimSpace(*patch.Frequency)
		if f == "" {
			return nil, fmt.Errorf("%w: frequency required", ErrConflict)
		}
		cur.Frequency = f
	}
	if patch.Retention != nil {
		if *patch.Retention < 0 {
			return nil, fmt.Errorf("%w: retention must be >= 0", ErrConflict)
		}
		cur.Retention = *patch.Retention
	}
	if patch.Description != nil {
		cur.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.Container != nil {
		cur.Container = strings.TrimSpace(*patch.Container)
	}
	if patch.DBUser != nil {
		u := strings.TrimSpace(*patch.DBUser)
		if u != "" {
			cur.DBUser = u
		}
	}
	if patch.DBName != nil {
		n := strings.TrimSpace(*patch.DBName)
		if n != "" {
			cur.DBName = n
		}
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE instance_settings SET
			backup_enabled=$1, backup_frequency=$2, backup_retention=$3,
			backup_description=$4, backup_container=$5, backup_db_user=$6, backup_db_name=$7,
			updated_at=NOW()
		WHERE id = 1
	`, cur.Enabled, cur.Frequency, cur.Retention, cur.Description, cur.Container, cur.DBUser, cur.DBName)
	if err != nil {
		return nil, err
	}
	return s.GetInstanceBackupConfig(ctx)
}

func (s *Store) ListInstanceBackupExecutions(ctx context.Context, limit int) ([]BackupExecution, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, scheduled_backup_id, resource_type, resource_id, status, size_bytes, filename,
			COALESCE(s3_uploaded,FALSE), COALESCE(s3_key,''), error_message, started_at, finished_at
		FROM backup_executions
		WHERE resource_type=$1 AND resource_id=$2
		ORDER BY started_at DESC
		LIMIT $3
	`, InstanceBackupResourceType, InstanceBackupResourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupExecution
	for rows.Next() {
		var b BackupExecution
		if err := rows.Scan(
			&b.ID, &b.TeamID, &b.ScheduledBackupID, &b.ResourceType, &b.ResourceID, &b.Status, &b.SizeBytes, &b.Filename,
			&b.S3Uploaded, &b.S3Key, &b.ErrorMessage, &b.StartedAt, &b.FinishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) CreateInstanceBackupExecution(ctx context.Context, teamID uuid.UUID, filename string) (*BackupExecution, error) {
	return s.CreateBackupExecution(ctx, teamID, InstanceBackupResourceType, InstanceBackupResourceID, filename)
}

func (s *Store) InstanceBackupRanThisMinute(ctx context.Context, minute time.Time) (bool, error) {
	start := minute.UTC().Truncate(time.Minute)
	end := start.Add(time.Minute)
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM backup_executions
		WHERE resource_type=$1 AND resource_id=$2
			AND started_at >= $3 AND started_at < $4
			AND status IN ('running','finished','success')
	`, InstanceBackupResourceType, InstanceBackupResourceID, start, end).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// FirstTeamID returns any team id for system jobs that need a team FK.
func (s *Store) FirstTeamID(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT id FROM teams ORDER BY created_at ASC LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

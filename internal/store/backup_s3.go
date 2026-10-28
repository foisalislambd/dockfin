package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxBackupS3Destinations = 8

// ErrTooManyS3Destinations is returned when a schedule selects more storages than a backup run uploads.
var ErrTooManyS3Destinations = errors.New("at most 8 s3 storages")

// BackupS3Replica is one copy of a backup execution in one S3 storage.
type BackupS3Replica struct {
	ID          uuid.UUID  `json:"id"`
	S3StorageID *uuid.UUID `json:"s3_storage_id,omitempty"`
	StorageName string     `json:"storage_name"`
	Uploaded    bool       `json:"uploaded"`
	ObjectKey   string     `json:"object_key,omitempty"`
	Message     string     `json:"message,omitempty"`
}

// DedupeDestinationIDs keeps the first occurrence of each storage and rejects more than eight.
func DedupeDestinationIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) > maxBackupS3Destinations {
		return nil, ErrTooManyS3Destinations
	}
	return out, nil
}

// KeepPrimary returns the current primary when it is still selected, otherwise the first id.
func KeepPrimary(current *uuid.UUID, ids []uuid.UUID) *uuid.UUID {
	if current != nil {
		for _, id := range ids {
			if id == *current {
				kept := *current
				return &kept
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	first := ids[0]
	return &first
}

func (s *Store) replaceBackupDestinations(ctx context.Context, teamID, backupID uuid.UUID, ids []uuid.UUID) error {
	ids, err := DedupeDestinationIDs(ids)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current *uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT s3_storage_id FROM scheduled_backups WHERE id=$1 AND team_id=$2 FOR UPDATE
	`, backupID, teamID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	for _, id := range ids {
		var n int
		err = tx.QueryRow(ctx, `SELECT 1 FROM s3_storages WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&n)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_backup_s3_storages WHERE scheduled_backup_id=$1`, backupID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `
			INSERT INTO scheduled_backup_s3_storages (scheduled_backup_id, s3_storage_id) VALUES ($1,$2)
		`, backupID, id); err != nil {
			return err
		}
	}
	primary := KeepPrimary(current, ids)
	if _, err := tx.Exec(ctx, `
		UPDATE scheduled_backups SET s3_storage_id=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2
	`, backupID, teamID, primary); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) attachBackupDestinations(ctx context.Context, backups []ScheduledBackup) error {
	if len(backups) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(backups))
	for i := range backups {
		backups[i].S3StorageIDs = []uuid.UUID{}
		ids[i] = backups[i].ID
	}
	grouped, err := s.destinationIDsByBackup(ctx, ids)
	if err != nil {
		return err
	}
	for i := range backups {
		if got := grouped[backups[i].ID]; len(got) > 0 {
			backups[i].S3StorageIDs = got
		} else if backups[i].S3StorageID != nil {
			backups[i].S3StorageIDs = []uuid.UUID{*backups[i].S3StorageID}
		}
	}
	return nil
}

func (s *Store) attachRowDestinations(ctx context.Context, rows []ScheduledBackupRow) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		rows[i].S3StorageIDs = []uuid.UUID{}
		ids[i] = rows[i].ID
	}
	grouped, err := s.destinationIDsByBackup(ctx, ids)
	if err != nil {
		return err
	}
	for i := range rows {
		if got := grouped[rows[i].ID]; len(got) > 0 {
			rows[i].S3StorageIDs = got
		}
		if len(rows[i].S3StorageIDs) == 0 && rows[i].S3StorageID != nil {
			rows[i].S3StorageIDs = []uuid.UUID{*rows[i].S3StorageID}
		}
	}
	return nil
}

func (s *Store) destinationIDsByBackup(ctx context.Context, backupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT p.scheduled_backup_id, p.s3_storage_id, (p.s3_storage_id IS NOT DISTINCT FROM b.s3_storage_id) AS is_primary
		FROM scheduled_backup_s3_storages p
		JOIN scheduled_backups b ON b.id = p.scheduled_backup_id
		JOIN s3_storages st ON st.id = p.s3_storage_id
		WHERE p.scheduled_backup_id = ANY($1)
		ORDER BY p.scheduled_backup_id, is_primary DESC, st.name, p.s3_storage_id
	`, backupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]uuid.UUID{}
	for rows.Next() {
		var backupID, storageID uuid.UUID
		var primary bool
		if err := rows.Scan(&backupID, &storageID, &primary); err != nil {
			return nil, err
		}
		out[backupID] = append(out[backupID], storageID)
	}
	return out, rows.Err()
}

// RecordBackupS3Replica stores the result of copying one execution to one storage.
func (s *Store) RecordBackupS3Replica(ctx context.Context, execID, storageID uuid.UUID, name string, uploaded bool, objectKey, message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	if len(name) > 200 {
		name = name[:200]
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO backup_s3_replicas (execution_id, s3_storage_id, storage_name, uploaded, object_key, message)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (execution_id, s3_storage_id) WHERE s3_storage_id IS NOT NULL
		DO UPDATE SET storage_name=EXCLUDED.storage_name, uploaded=EXCLUDED.uploaded,
			object_key=EXCLUDED.object_key, message=EXCLUDED.message
	`, execID, storageID, name, uploaded, objectKey, message)
	return err
}

// RefreshBackupS3Summary sets the execution flags from its replica rows.
// Uploaded is true only when every destination received the file.
func (s *Store) RefreshBackupS3Summary(ctx context.Context, execID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE backup_executions e SET
			s3_uploaded = COALESCE((
				SELECT COUNT(*) > 0 AND BOOL_AND(r.uploaded)
				FROM backup_s3_replicas r WHERE r.execution_id = e.id
			), FALSE),
			s3_key = COALESCE((
				SELECT r.object_key FROM backup_s3_replicas r
				WHERE r.execution_id = e.id AND r.uploaded AND r.object_key <> ''
				ORDER BY r.created_at
				LIMIT 1
			), '')
		WHERE e.id = $1
	`, execID)
	return err
}

// MarkBackupS3Incomplete clears the uploaded flag when any destination failed or its result was not saved.
func (s *Store) MarkBackupS3Incomplete(ctx context.Context, execID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `UPDATE backup_executions SET s3_uploaded=FALSE WHERE id=$1`, execID)
	return err
}

// NoteBackupS3Failure records destination errors on a finished execution without changing its status.
func (s *Store) NoteBackupS3Failure(ctx context.Context, execID uuid.UUID, message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE backup_executions
		SET error_message = CASE WHEN error_message = '' THEN $2 ELSE error_message END
		WHERE id=$1 AND status='finished'
	`, execID, message)
	return err
}

func (s *Store) attachBackupReplicas(ctx context.Context, execs []BackupExecution) error {
	if len(execs) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(execs))
	for i := range execs {
		ids[i] = execs[i].ID
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, execution_id, s3_storage_id, storage_name, uploaded, object_key, message
		FROM backup_s3_replicas
		WHERE execution_id = ANY($1)
		ORDER BY created_at
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	grouped := map[uuid.UUID][]BackupS3Replica{}
	for rows.Next() {
		var execID uuid.UUID
		var row BackupS3Replica
		if err := rows.Scan(&row.ID, &execID, &row.S3StorageID, &row.StorageName, &row.Uploaded, &row.ObjectKey, &row.Message); err != nil {
			return err
		}
		grouped[execID] = append(grouped[execID], row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range execs {
		if got := grouped[execs[i].ID]; len(got) > 0 {
			execs[i].S3Replicas = got
		}
	}
	return nil
}

// DestinationIDs returns a copy of the storages a schedule uploads to, primary first.
func DestinationIDs(primary *uuid.UUID, ids []uuid.UUID) []uuid.UUID {
	if len(ids) > 0 {
		out := make([]uuid.UUID, len(ids))
		copy(out, ids)
		return out
	}
	if primary != nil {
		return []uuid.UUID{*primary}
	}
	return nil
}

type sqlExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// PromoteBackupPrimaries gives a primary to schedules that still have destinations after a storage was removed.
func (s *Store) PromoteBackupPrimaries(ctx context.Context, teamID uuid.UUID) error {
	return promoteBackupPrimaries(ctx, s.Pool, teamID)
}

func promoteBackupPrimaries(ctx context.Context, exec sqlExecer, teamID uuid.UUID) error {
	_, err := exec.Exec(ctx, `
		UPDATE scheduled_backups b
		SET s3_storage_id = (
			SELECT p.s3_storage_id
			FROM scheduled_backup_s3_storages p
			JOIN s3_storages st ON st.id = p.s3_storage_id AND st.team_id = b.team_id
			WHERE p.scheduled_backup_id = b.id
			ORDER BY st.name, p.s3_storage_id
			LIMIT 1
		)
		WHERE b.team_id = $1
		  AND b.s3_storage_id IS NULL
		  AND EXISTS (
			SELECT 1 FROM scheduled_backup_s3_storages p WHERE p.scheduled_backup_id = b.id
		  )
	`, teamID)
	return err
}

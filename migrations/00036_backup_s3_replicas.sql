-- +goose Up
-- A schedule can copy one backup to several S3 storages. s3_storage_id stays the primary destination.
CREATE TABLE IF NOT EXISTS scheduled_backup_s3_storages (
    scheduled_backup_id UUID NOT NULL REFERENCES scheduled_backups(id) ON DELETE CASCADE,
    s3_storage_id       UUID NOT NULL REFERENCES s3_storages(id) ON DELETE CASCADE,
    PRIMARY KEY (scheduled_backup_id, s3_storage_id)
);

CREATE TABLE IF NOT EXISTS backup_s3_replicas (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id   UUID NOT NULL REFERENCES backup_executions(id) ON DELETE CASCADE,
    s3_storage_id  UUID REFERENCES s3_storages(id) ON DELETE SET NULL,
    storage_name   TEXT NOT NULL DEFAULT '',
    uploaded       BOOLEAN NOT NULL DEFAULT FALSE,
    object_key     TEXT NOT NULL DEFAULT '',
    message        TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS backup_s3_replicas_exec_storage_uidx
    ON backup_s3_replicas (execution_id, s3_storage_id)
    WHERE s3_storage_id IS NOT NULL;

INSERT INTO scheduled_backup_s3_storages (scheduled_backup_id, s3_storage_id)
SELECT b.id, b.s3_storage_id
FROM scheduled_backups b
JOIN s3_storages st ON st.id = b.s3_storage_id
WHERE b.s3_storage_id IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO backup_s3_replicas (execution_id, s3_storage_id, storage_name, uploaded, object_key)
SELECT e.id, st.id, st.name, TRUE, COALESCE(e.s3_key, '')
FROM backup_executions e
JOIN scheduled_backups b ON b.id = e.scheduled_backup_id
JOIN s3_storages st ON st.id = b.s3_storage_id
WHERE e.s3_uploaded = TRUE
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS backup_s3_replicas;
DROP TABLE IF EXISTS scheduled_backup_s3_storages;

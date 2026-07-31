-- +goose Up
ALTER TABLE instance_settings
    ADD COLUMN IF NOT EXISTS backup_configured BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS backup_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS backup_frequency TEXT NOT NULL DEFAULT '0 0 * * *',
    ADD COLUMN IF NOT EXISTS backup_retention INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS backup_description TEXT NOT NULL DEFAULT 'Goolify database',
    ADD COLUMN IF NOT EXISTS backup_db_user TEXT NOT NULL DEFAULT 'goolify',
    ADD COLUMN IF NOT EXISTS backup_db_name TEXT NOT NULL DEFAULT 'goolify',
    ADD COLUMN IF NOT EXISTS backup_container TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE instance_settings
    DROP COLUMN IF EXISTS backup_configured,
    DROP COLUMN IF EXISTS backup_enabled,
    DROP COLUMN IF EXISTS backup_frequency,
    DROP COLUMN IF EXISTS backup_retention,
    DROP COLUMN IF EXISTS backup_description,
    DROP COLUMN IF EXISTS backup_db_user,
    DROP COLUMN IF EXISTS backup_db_name,
    DROP COLUMN IF EXISTS backup_container;

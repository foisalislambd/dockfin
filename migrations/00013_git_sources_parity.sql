-- +goose Up
ALTER TABLE git_sources
    ADD COLUMN IF NOT EXISTS organization TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS custom_user TEXT NOT NULL DEFAULT 'git',
    ADD COLUMN IF NOT EXISTS custom_port INT NOT NULL DEFAULT 22,
    ADD COLUMN IF NOT EXISTS is_system_wide BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS private_key_id UUID REFERENCES private_keys(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS applications_git_source_id_idx ON applications(git_source_id);
CREATE INDEX IF NOT EXISTS applications_private_key_id_idx ON applications(private_key_id);

-- +goose Down
DROP INDEX IF EXISTS applications_private_key_id_idx;
DROP INDEX IF EXISTS applications_git_source_id_idx;
ALTER TABLE applications DROP COLUMN IF EXISTS private_key_id;
ALTER TABLE git_sources
    DROP COLUMN IF EXISTS is_system_wide,
    DROP COLUMN IF EXISTS custom_port,
    DROP COLUMN IF EXISTS custom_user,
    DROP COLUMN IF EXISTS organization;

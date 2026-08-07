-- +goose Up
ALTER TABLE deployments
    ALTER COLUMN application_id DROP NOT NULL;

ALTER TABLE deployments
    ADD COLUMN IF NOT EXISTS service_id UUID REFERENCES services(id) ON DELETE CASCADE;

UPDATE deployments SET application_id = application_id WHERE application_id IS NOT NULL;

ALTER TABLE deployments
    DROP CONSTRAINT IF EXISTS deployments_resource_chk;

ALTER TABLE deployments
    ADD CONSTRAINT deployments_resource_chk CHECK (
        (application_id IS NOT NULL AND service_id IS NULL)
        OR (application_id IS NULL AND service_id IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS deployments_service_id_idx ON deployments(service_id);

-- +goose Down
DROP INDEX IF EXISTS deployments_service_id_idx;
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_resource_chk;
ALTER TABLE deployments DROP COLUMN IF EXISTS service_id;
-- Cannot safely restore NOT NULL if service rows exist; leave nullable on down.

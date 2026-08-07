-- +goose Up
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS compose_prepare BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN applications.compose_prepare IS
    'When true (default), Dockfin adapts docker-compose for Traefik/network/ports. When false, deploy the repo compose file as-is.';

-- +goose Down
ALTER TABLE applications
    DROP COLUMN IF EXISTS compose_prepare;

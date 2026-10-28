-- +goose Up
ALTER TABLE databases DROP CONSTRAINT IF EXISTS databases_engine_check;
ALTER TABLE databases ADD CONSTRAINT databases_engine_check CHECK (engine IN (
    'postgresql', 'mysql', 'mariadb', 'mongodb',
    'redis', 'keydb', 'dragonfly', 'clickhouse', 'sqlite'
));

-- +goose Down
ALTER TABLE databases DROP CONSTRAINT IF EXISTS databases_engine_check;
ALTER TABLE databases ADD CONSTRAINT databases_engine_check CHECK (engine IN (
    'postgresql', 'mysql', 'mariadb', 'mongodb',
    'redis', 'keydb', 'dragonfly', 'clickhouse'
));

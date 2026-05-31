-- +goose Up
-- +goose StatementBegin

-- Adds expiry/rotation metadata to the secrets table (C5: secret expiry/rotation).
-- rotated_at: last time the value was rotated (NULL = never rotated since creation).
-- expires_at: operator-set hard deadline; NULL = no expiry configured.
-- Both columns are nullable so the migration is non-destructive on existing rows.
ALTER TABLE secrets ADD COLUMN rotated_at  TIMESTAMP;
ALTER TABLE secrets ADD COLUMN expires_at  TIMESTAMP;

CREATE INDEX idx_secrets_expires ON secrets(expires_at) WHERE expires_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_secrets_expires;
-- SQLite has no DROP COLUMN before 3.35 — the columns become inert on rollback.
-- A full table-rebuild is left as a manual migration step if needed.

-- +goose StatementEnd

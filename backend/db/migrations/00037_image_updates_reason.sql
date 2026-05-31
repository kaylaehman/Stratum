-- +goose Up
-- +goose StatementBegin

-- Add unknown_reason to image_updates so callers can surface WHY a digest
-- comparison could not be made (locally-built image, private registry,
-- rate-limited, network error, etc.) rather than a bare "unknown".
ALTER TABLE image_updates ADD COLUMN unknown_reason TEXT NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite does not support DROP COLUMN before 3.35; omit for safety.
-- The column is additive and harmless when the migration is rolled back.
-- +goose StatementEnd

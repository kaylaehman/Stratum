-- +goose Up
-- +goose StatementBegin

-- feature_flags: per-feature enable toggles (FEATURES.md "Feature Flags").
-- Rows are seeded on demand by the service from the known-flag list; a missing
-- row means "use the built-in default".
CREATE TABLE feature_flags (
    key           TEXT PRIMARY KEY,
    enabled       INTEGER NOT NULL DEFAULT 0,
    configured_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE feature_flags;
-- +goose StatementEnd

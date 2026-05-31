-- +goose Up
-- stack_env stores per-(node, project) environment variable overrides for live
-- Compose stacks. The value column holds the plaintext AES-256-GCM sealed by
-- the backend; secret_id is a foreign-key-style pointer to the secrets vault.
-- Exactly one of (value, secret_id) is non-empty per row.
CREATE TABLE IF NOT EXISTS stack_env (
    node_id      TEXT    NOT NULL,
    project_name TEXT    NOT NULL,
    key          TEXT    NOT NULL,
    value        TEXT    NOT NULL DEFAULT '',   -- AES-sealed plaintext; empty when secret_id set
    secret_id    TEXT    NOT NULL DEFAULT '',   -- secrets vault id; empty when value set
    PRIMARY KEY (node_id, project_name, key)
);

-- +goose Down
DROP TABLE IF EXISTS stack_env;

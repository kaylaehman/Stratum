-- +goose Up
-- +goose StatementBegin

-- sso_config: per-container access-control settings (Feature F2). This is
-- CONFIGURATION ONLY — the enforcing auth gateway (a reverse proxy in front of
-- the container port) is a follow-on that consumes these rows. Keyed by
-- (node_id, container_name) since a container's docker id churns on recreate.
-- client_secret_encrypted is AES-sealed; the plaintext is never returned.
CREATE TABLE sso_config (
    id                      TEXT PRIMARY KEY,
    node_id                 TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    container_name          TEXT NOT NULL,
    enabled                 INTEGER NOT NULL DEFAULT 0,
    method                  TEXT NOT NULL DEFAULT 'local', -- local | totp | oidc | forward
    provider_url            TEXT NOT NULL DEFAULT '',
    client_id               TEXT NOT NULL DEFAULT '',
    client_secret_encrypted BLOB,
    allowed_groups          TEXT NOT NULL DEFAULT '[]',
    session_duration_secs   INTEGER NOT NULL DEFAULT 86400,
    updated_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(node_id, container_name)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sso_config;
-- +goose StatementEnd

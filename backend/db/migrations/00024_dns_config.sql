-- +goose Up
-- +goose StatementBegin

-- dns_config: per-node DNS admin endpoint + optional API token (Feature F3).
-- Mirrors proxy_config; the detected tool comes from the container inventory.
CREATE TABLE dns_config (
    node_id         TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    endpoint        TEXT NOT NULL DEFAULT '',
    token_encrypted BLOB,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE dns_config;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- proxy_config: per-node reverse-proxy admin endpoint + optional API token
-- (Feature F1). The detected tool comes from the container inventory; this row
-- only stores how to REACH its admin API for live rule listing. token_encrypted
-- is AES-sealed; the plaintext is never stored or returned.
CREATE TABLE proxy_config (
    node_id         TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    endpoint        TEXT NOT NULL DEFAULT '',
    token_encrypted BLOB,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE proxy_config;
-- +goose StatementEnd

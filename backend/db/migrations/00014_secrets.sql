-- +goose Up
-- +goose StatementBegin

-- secret_groups + secrets: encrypted env-var vault (Feature 12). Secret values
-- are AES-256-GCM sealed (never stored plaintext); only key names are listed
-- without an explicit, audited reveal.
CREATE TABLE secret_groups (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE secrets (
    id              TEXT PRIMARY KEY,
    group_id        TEXT NOT NULL REFERENCES secret_groups(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    value_encrypted BLOB NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(group_id, key)
);
CREATE INDEX idx_secrets_group ON secrets(group_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE secrets;
DROP TABLE secret_groups;
-- +goose StatementEnd

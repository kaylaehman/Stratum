-- +goose Up
-- +goose StatementBegin

-- user_totp: per-user TOTP 2FA (Feature 7). The secret is AES-sealed; recovery
-- codes are stored as bcrypt hashes (consumed on use). Removed with the user.
CREATE TABLE user_totp (
    user_id          TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_encrypted BLOB NOT NULL,
    enabled          INTEGER NOT NULL DEFAULT 0,
    recovery_codes   TEXT NOT NULL DEFAULT '[]',
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_totp;
-- +goose StatementEnd

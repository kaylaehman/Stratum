-- +goose Up
-- +goose StatementBegin

-- chat_config: inbound chat-bot configuration (Feature F8), single row. The bot
-- token is AES-sealed; allowed_chats is a JSON array of authorized chat IDs
-- (only these may issue commands).
CREATE TABLE chat_config (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    provider        TEXT NOT NULL DEFAULT 'telegram',
    token_encrypted BLOB,
    allowed_chats   TEXT NOT NULL DEFAULT '[]',
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE chat_config;
-- +goose StatementEnd

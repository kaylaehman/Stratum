-- +goose Up
-- +goose StatementBegin

-- ai_config: single-row provider configuration for the AI Assistant
-- (Feature 31). The Claude API key is AES-sealed (api_key_encrypted) and never
-- stored or returned in plaintext. The CHECK(id = 1) enforces a single row.
CREATE TABLE ai_config (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    provider          TEXT NOT NULL DEFAULT '',   -- ollama | claude | ''
    ollama_base_url   TEXT NOT NULL DEFAULT '',
    ollama_model      TEXT NOT NULL DEFAULT '',
    claude_model      TEXT NOT NULL DEFAULT '',
    api_key_encrypted BLOB,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE ai_config;
-- +goose StatementEnd

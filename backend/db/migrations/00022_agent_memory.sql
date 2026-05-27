-- +goose Up
-- +goose StatementBegin

-- agent_memory: persistent per-scope context the AI assistant uses (Feature F9).
-- scope is global | node | container; scope_id is '' for global, else the node
-- or container id. source records who created it (user | ai | observed); an
-- AI-proposed memory stays confirmed=0 until a user accepts it, and only
-- confirmed memories are injected into the assistant's context.
CREATE TABLE agent_memory (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL,            -- global | node | container
    scope_id   TEXT NOT NULL DEFAULT '',
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    source     TEXT NOT NULL DEFAULT 'user', -- user | ai | observed
    confirmed  INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(scope, scope_id, key)
);
CREATE INDEX idx_agent_memory_scope ON agent_memory(scope, scope_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE agent_memory;
-- +goose StatementEnd

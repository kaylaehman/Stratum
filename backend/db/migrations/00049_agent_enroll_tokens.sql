-- +goose Up
-- +goose StatementBegin
-- Single-use, short-lived tokens authorizing one agent enrollment (CSR signing)
-- for a specific node. Only the SHA-256 hash of the token is stored.
CREATE TABLE agent_enroll_tokens (
  id          TEXT PRIMARY KEY,
  node_id     TEXT NOT NULL,
  token_hash  TEXT NOT NULL UNIQUE,
  action      TEXT NOT NULL,
  created_by  TEXT,
  created_at  TEXT NOT NULL,
  expires_at  TEXT NOT NULL,
  used_at     TEXT
);

CREATE INDEX agent_enroll_tokens_node ON agent_enroll_tokens (node_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_enroll_tokens;
-- +goose StatementEnd

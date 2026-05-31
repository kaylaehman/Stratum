-- +goose Up
-- config_versions stores every tracked snapshot of a config file on a node.
-- Content is capped at 1MB before insert (enforced in the service layer).
CREATE TABLE IF NOT EXISTS config_versions (
    id          TEXT PRIMARY KEY,
    node_id     TEXT NOT NULL,
    path        TEXT NOT NULL,
    content     TEXT NOT NULL,
    hash        TEXT NOT NULL,    -- SHA-256 hex of content
    author      TEXT NOT NULL,    -- user ID who triggered the snapshot
    created_at  TEXT NOT NULL,
    -- Queries by (node_id, path) ordered newest-first are the hot path.
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_config_versions_node_path
    ON config_versions (node_id, path, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_config_versions_node_path;
DROP TABLE IF EXISTS config_versions;

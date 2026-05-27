-- +goose Up
-- +goose StatementBegin

-- snapshots: rollback checkpoints for a container (Feature 15/17). Captured
-- before a recreate/update, or manually. spec_json is the full container
-- create-spec (Config + HostConfig + network endpoints) so the container can be
-- reproduced even after the original is gone. Keyed for listing by
-- (node_id, container_name) because a recreate replaces the container's docker
-- id (and thus its inventory row) while the name is stable.
CREATE TABLE snapshots (
    id             TEXT PRIMARY KEY,
    container_id   TEXT NOT NULL DEFAULT '',   -- app uuid at capture time (best-effort; may go stale)
    node_id        TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    container_name TEXT NOT NULL,
    reason         TEXT NOT NULL DEFAULT '',
    image_ref      TEXT NOT NULL DEFAULT '',
    image_digest   TEXT NOT NULL DEFAULT '',
    spec_json      TEXT NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_snapshots_container ON snapshots(node_id, container_name, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE snapshots;
-- +goose StatementEnd

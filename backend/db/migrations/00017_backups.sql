-- +goose Up
-- +goose StatementBegin

-- backups: history of volume/VM backup jobs (Feature 28). Rows are created
-- "running" and updated to ok/error when the async job finishes.
CREATE TABLE backups (
    id          TEXT PRIMARY KEY,
    node_id     TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,            -- volume | proxmox
    target      TEXT NOT NULL,            -- volume name or vmid
    dest_path   TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    status      TEXT NOT NULL,            -- running | ok | error
    error       TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP
);
CREATE INDEX idx_backups_started ON backups(started_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE backups;
-- +goose StatementEnd

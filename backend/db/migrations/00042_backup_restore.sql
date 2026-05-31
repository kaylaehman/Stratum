-- +goose Up
-- +goose StatementBegin

-- backup_verify_results: records of restore-drill integrity checks (C1).
-- Each row captures the outcome of restoring the newest archive for a node
-- to a scratch location and stat-ing the result. Rows are append-only.
CREATE TABLE backup_verify_results (
    id           TEXT PRIMARY KEY,
    node_id      TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    backup_id    TEXT NOT NULL,            -- FK into backups.id (soft ref)
    archive_path TEXT NOT NULL,
    scratch_dir  TEXT NOT NULL DEFAULT '',
    file_count   INTEGER NOT NULL DEFAULT 0,
    total_bytes  INTEGER NOT NULL DEFAULT 0,
    passed       INTEGER NOT NULL DEFAULT 0, -- 0=false 1=true
    error        TEXT NOT NULL DEFAULT '',
    verified_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_backup_verify_node ON backup_verify_results(node_id, verified_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS backup_verify_results;
-- +goose StatementEnd

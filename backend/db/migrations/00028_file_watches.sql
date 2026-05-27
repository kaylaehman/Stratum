-- +goose Up
-- +goose StatementBegin

-- file_watches: configurable paths to monitor for out-of-band changes
-- (Feature 22). Real-time inotify needs the agent; without it Stratum polls
-- these paths over SSH for recently-modified files.
CREATE TABLE file_watches (
    id         TEXT PRIMARY KEY,
    node_id    TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    recursive  INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(node_id, path)
);

-- file_events: detected changes under watched paths.
CREATE TABLE file_events (
    id          TEXT PRIMARY KEY,
    node_id     TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    path        TEXT NOT NULL,
    event_type  TEXT NOT NULL DEFAULT 'modified',
    detected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_file_events_node ON file_events(node_id, detected_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE file_events;
DROP TABLE file_watches;
-- +goose StatementEnd

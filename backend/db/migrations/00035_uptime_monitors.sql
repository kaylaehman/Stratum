-- +goose Up
-- +goose StatementBegin

-- uptime_monitors: user-configured endpoint checks (http/tcp/icmp).
-- node_id is nullable — NULL means the check originates from the backend host.
CREATE TABLE uptime_monitors (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('http','tcp','icmp')),
    target           TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL DEFAULT 60,
    timeout_ms       INTEGER NOT NULL DEFAULT 5000,
    expected         TEXT NOT NULL DEFAULT '',
    enabled          INTEGER NOT NULL DEFAULT 1,
    node_id          TEXT REFERENCES nodes(id) ON DELETE SET NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- uptime_results: time-series results for each monitor check.
-- status: 'up' | 'down' | 'degraded'
CREATE TABLE uptime_results (
    id               TEXT PRIMARY KEY,
    monitor_id       TEXT NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    checked_at       TIMESTAMP NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('up','down','degraded')),
    response_time_ms INTEGER NOT NULL DEFAULT 0,
    error            TEXT NOT NULL DEFAULT ''
);

CREATE INDEX uptime_results_monitor_checked ON uptime_results (monitor_id, checked_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uptime_results_monitor_checked;
DROP TABLE IF EXISTS uptime_results;
DROP TABLE IF EXISTS uptime_monitors;
-- +goose StatementEnd

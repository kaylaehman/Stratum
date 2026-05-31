-- +goose Up
-- +goose StatementBegin

-- cve_schedules: recurring CVE scan schedules (Task 2, Feature 20 extension).
-- target_type: "node" (all containers on the node) | "container" (single container).
-- target_id: the node or container id.
-- interval_seconds: scan interval (minimum 3600 = 1 hour; UI enforces this).
-- enabled: whether the schedule is active; disabled schedules are skipped by the runner.
-- last_run_at: last time the schedule fired (NULL = never run).
CREATE TABLE cve_schedules (
    id               TEXT PRIMARY KEY,
    target_type      TEXT NOT NULL DEFAULT 'node',
    target_id        TEXT NOT NULL,
    label            TEXT NOT NULL DEFAULT '',
    interval_seconds INTEGER NOT NULL DEFAULT 86400,
    enabled          INTEGER NOT NULL DEFAULT 1,
    created_by       TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_run_at      TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cve_schedules_enabled ON cve_schedules (enabled);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cve_schedules;
-- +goose StatementEnd

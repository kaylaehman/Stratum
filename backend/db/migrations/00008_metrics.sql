-- +goose Up
-- +goose StatementBegin

-- resource_samples: a 15s-polled time series of per-container CPU/RAM/disk-IO
-- (Feature 9). Regenerable telemetry (not an audit trail); pruned on a retention
-- window by the sampler. cpu_pct is a percentage (0-100*cores); mem/disk are
-- bytes; mem_limit_bytes is the container's memory limit (0 = unlimited) for
-- RAM-percentage spike detection.
CREATE TABLE resource_samples (
    id               TEXT PRIMARY KEY,
    container_id     TEXT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    node_id          TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    cpu_pct          REAL NOT NULL,
    mem_bytes        INTEGER NOT NULL,
    mem_limit_bytes  INTEGER NOT NULL,
    disk_read_bytes  INTEGER NOT NULL,
    disk_write_bytes INTEGER NOT NULL,
    sampled_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_resource_samples_lookup ON resource_samples(container_id, sampled_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE resource_samples;
-- +goose StatementEnd

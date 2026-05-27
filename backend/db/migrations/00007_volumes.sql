-- +goose Up
-- +goose StatementBegin

-- volume_samples: a daily-sampled size/refcount time series per volume, for the
-- Volume Health size-trend chart (Feature 7). Volumes themselves are listed live
-- from the daemon (seed-on-query); only the historical trend is persisted.
CREATE TABLE volume_samples (
    id          TEXT PRIMARY KEY,
    node_id     TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    volume_name TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL,   -- -1 when the daemon cannot report it
    ref_count   INTEGER NOT NULL,   -- -1 when unknown
    sampled_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_volume_samples_lookup ON volume_samples(node_id, volume_name, sampled_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE volume_samples;
-- +goose StatementEnd

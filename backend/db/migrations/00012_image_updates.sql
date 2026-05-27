-- +goose Up
-- +goose StatementBegin

-- image_updates: cached image update-availability per container (Feature 15,
-- detection half). Populated by a seed-on-query check against the registry
-- (slow/rate-limited, so cached with a TTL). status ∈ up_to_date |
-- update_available | unknown. CASCADE with the container/node.
CREATE TABLE image_updates (
    container_id   TEXT PRIMARY KEY REFERENCES containers(id) ON DELETE CASCADE,
    node_id        TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    image          TEXT NOT NULL,
    status         TEXT NOT NULL,
    current_digest TEXT NOT NULL DEFAULT '',
    latest_digest  TEXT NOT NULL DEFAULT '',
    checked_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE image_updates;
-- +goose StatementEnd

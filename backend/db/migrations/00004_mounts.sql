-- +goose Up
-- +goose StatementBegin

-- mounts: the bind-mount/volume index for forward (by container), reverse (by
-- host path), and shared (multi-container) lookups. Populated from container
-- inspect; rows cascade-delete with their node and container.
CREATE TABLE mounts (
    id                TEXT PRIMARY KEY,            -- UUID
    node_id           TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    container_id      TEXT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    type              TEXT NOT NULL,               -- bind | volume | tmpfs | ...
    source            TEXT NOT NULL,               -- host path (bind) or driver _data path (volume); "" for tmpfs
    normalized_source TEXT NOT NULL,               -- path.Clean(source); the BIND reverse-lookup key
    volume_name       TEXT,                        -- Docker volume Name (type=volume only); canonical volume identity
    destination       TEXT NOT NULL,               -- container path
    rw                INTEGER NOT NULL,            -- from inspect RW bool (NOT the Mode string)
    UNIQUE(container_id, destination)
);
CREATE INDEX idx_mounts_node_src  ON mounts(node_id, normalized_source);
CREATE INDEX idx_mounts_node_vol  ON mounts(node_id, volume_name);
CREATE INDEX idx_mounts_container ON mounts(container_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE mounts;
-- +goose StatementEnd

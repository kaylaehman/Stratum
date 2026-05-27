-- +goose Up
-- +goose StatementBegin

-- wol_config: optional Wake-on-LAN settings per node (Feature 6). One row per
-- node; removed when the node is deleted (CASCADE).
CREATE TABLE wol_config (
    node_id   TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    mac       TEXT NOT NULL,
    broadcast TEXT NOT NULL DEFAULT '255.255.255.255',
    port      INTEGER NOT NULL DEFAULT 9
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE wol_config;
-- +goose StatementEnd

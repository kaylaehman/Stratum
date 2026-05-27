-- +goose Up
-- +goose StatementBegin

-- certs: TLS certificates discovered on a node's filesystem (Feature F4).
-- Monitor-only — Stratum surfaces expiry, it never issues certs. Rows are
-- replaced wholesale per node on each scan. sans is a JSON array.
CREATE TABLE certs (
    id           TEXT PRIMARY KEY,
    node_id      TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    source       TEXT NOT NULL DEFAULT 'filesystem',
    domain       TEXT NOT NULL DEFAULT '',
    sans         TEXT NOT NULL DEFAULT '[]',
    issuer       TEXT NOT NULL DEFAULT '',
    path         TEXT NOT NULL DEFAULT '',
    not_before   TIMESTAMP,
    not_after    TIMESTAMP,
    last_checked TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_certs_node ON certs(node_id);
CREATE INDEX idx_certs_expiry ON certs(not_after);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE certs;
-- +goose StatementEnd

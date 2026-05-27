-- +goose Up
-- +goose StatementBegin

CREATE TABLE container_security (
    container_id        TEXT PRIMARY KEY REFERENCES containers(id) ON DELETE CASCADE,
    node_id             TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    privileged          INTEGER NOT NULL DEFAULT 0,
    cap_add_all         INTEGER NOT NULL DEFAULT 0,
    dangerous_caps      TEXT NOT NULL DEFAULT '[]',   -- JSON array of bare cap names
    seccomp_unconfined  INTEGER NOT NULL DEFAULT 0,
    apparmor_unconfined INTEGER NOT NULL DEFAULT 0,
    devices             TEXT NOT NULL DEFAULT '[]',   -- JSON array
    userns_host         INTEGER NOT NULL DEFAULT 0,
    pid_host            INTEGER NOT NULL DEFAULT 0,
    net_host            INTEGER NOT NULL DEFAULT 0,
    runs_as_root        INTEGER NOT NULL DEFAULT 0,
    run_uid             INTEGER,
    scanned_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE port_exposures (
    id              TEXT PRIMARY KEY,
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    container_id    TEXT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    host_ip         TEXT NOT NULL,
    host_port       INTEGER NOT NULL,
    container_port  INTEGER NOT NULL,
    protocol        TEXT NOT NULL,
    interface_class TEXT NOT NULL,                -- all | loopback | external
    is_new          INTEGER NOT NULL DEFAULT 0,   -- durable until acknowledged/dismissed
    notified_at     TIMESTAMP,                    -- set by Feature 26 after dispatch
    first_seen      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(container_id, host_ip, host_port, protocol)
);
CREATE INDEX idx_portexp_node ON port_exposures(node_id);

CREATE TABLE security_acknowledgements (
    id              TEXT PRIMARY KEY,
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    container_id    TEXT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    flag_type       TEXT NOT NULL,                -- privileged | cap | pid_host | net_host | root | port | ...
    flag_key        TEXT NOT NULL,                -- e.g. SYS_ADMIN, 0.0.0.0:8080, ""
    acknowledged_by TEXT REFERENCES users(id),
    note            TEXT,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(container_id, flag_type, flag_key)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE security_acknowledgements;
DROP TABLE port_exposures;
DROP TABLE container_security;
-- +goose StatementEnd

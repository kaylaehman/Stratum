-- +goose Up
-- +goose StatementBegin

-- nodes: any registered Linux host (proxmox | standalone | ssh). Credentials are
-- stored only as an AES-256-GCM sealed blob; never plaintext. last_error always
-- holds a sanitized category, never a raw transport error (SP1 §5.6).
CREATE TABLE nodes (
    id                    TEXT PRIMARY KEY,                -- UUID
    name                  TEXT NOT NULL,
    type                  TEXT NOT NULL,                   -- proxmox | standalone | ssh
    host                  TEXT NOT NULL,
    port                  INTEGER NOT NULL DEFAULT 22,     -- SSH port
    auth_method           TEXT NOT NULL,                   -- ssh_password | ssh_key
    os_type               TEXT,                            -- debian|ubuntu|rhel|arch|alpine|other
    capabilities_json     TEXT NOT NULL DEFAULT '{}',      -- {proxmox,docker,agent,systemd,cron, proxmox_auth_status}
    credentials_encrypted BLOB NOT NULL,                   -- crypto.Seal(JSON of NodeCredentials)
    credentials_version   INTEGER NOT NULL DEFAULT 1,      -- sealed-blob schema version
    ssh_host_key          TEXT,                            -- accepted host key (knownhosts line); TOFU-pinned
    proxmox_endpoint      TEXT,                            -- optional, e.g. https://host:8006
    proxmox_tls_insecure  INTEGER NOT NULL DEFAULT 0,      -- bool: allow self-signed (default OFF)
    docker_endpoint       TEXT,                            -- optional, tcp://host:2376
    status                TEXT NOT NULL DEFAULT 'unknown', -- ok|unreachable|error|unknown
    last_error            TEXT,                            -- sanitized category only, never raw (§5.6)
    last_seen             TIMESTAMP,
    created_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_nodes_type   ON nodes(type);
CREATE INDEX idx_nodes_status ON nodes(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE nodes;
-- +goose StatementEnd

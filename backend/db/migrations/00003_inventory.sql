-- +goose Up
-- +goose StatementBegin

-- vms: Proxmox guests — QEMU VMs and LXC containers in one table (kind column).
CREATE TABLE vms (
    id            TEXT PRIMARY KEY,                 -- UUID
    node_id       TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,                    -- qemu | lxc
    proxmox_vmid  INTEGER NOT NULL,
    proxmox_node  TEXT NOT NULL,                    -- cluster node name owning the guest
    name          TEXT NOT NULL,
    status        TEXT NOT NULL,                    -- running | stopped | paused | error | unknown
    os_type       TEXT,
    stale         INTEGER NOT NULL DEFAULT 0,
    gone_since    TIMESTAMP,                        -- set when first absent; removed after 2 polls
    last_seen     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(node_id, proxmox_vmid, kind)
);

CREATE TABLE containers (
    id              TEXT PRIMARY KEY,               -- UUID
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    docker_id       TEXT NOT NULL,                  -- full container ID
    name            TEXT NOT NULL,
    image           TEXT NOT NULL,                  -- image ref/tag as reported
    image_id        TEXT,                           -- local content-addressable ID (NOT a registry repo digest)
    status          TEXT NOT NULL,                  -- running | exited | paused | restarting | dead | created
    compose_project TEXT,
    stale           INTEGER NOT NULL DEFAULT 0,
    gone_since      TIMESTAMP,                       -- set when first absent; removed after 2 polls
    last_seen       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(node_id, docker_id)
);
CREATE INDEX idx_vms_node        ON vms(node_id);
CREATE INDEX idx_vms_node_kind   ON vms(node_id, kind);
CREATE INDEX idx_containers_node ON containers(node_id);
CREATE INDEX idx_containers_proj ON containers(compose_project);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE containers;
DROP TABLE vms;
-- +goose StatementEnd

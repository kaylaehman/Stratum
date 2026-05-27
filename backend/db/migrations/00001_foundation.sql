-- +goose Up
-- +goose StatementBegin

-- users: single-user MVP, multi-user-ready (role column present, enforced in feature 30).
CREATE TABLE users (
    id            TEXT PRIMARY KEY,                 -- UUID
    username      TEXT NOT NULL UNIQUE,
    email         TEXT,
    password_hash TEXT NOT NULL,                    -- bcrypt
    role          TEXT NOT NULL DEFAULT 'admin',    -- admin|operator|viewer
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- sessions: refresh-token store enabling revocation.
CREATE TABLE sessions (
    id            TEXT PRIMARY KEY,                 -- UUID, opaque refresh-token id
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_hash  TEXT NOT NULL,                    -- sha256 of the refresh token; never store raw
    user_agent    TEXT,
    ip            TEXT,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at    TIMESTAMP NOT NULL,
    revoked_at    TIMESTAMP
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

-- activity_log: append-only audit trail, written from day one.
-- Intentionally NO ON DELETE clause on user_id: deleting a user must not purge
-- their audit trail. With foreign_keys=ON, deleting a referenced user errors;
-- the feature-30 user-deletion flow must soft-delete / null this, never cascade.
CREATE TABLE activity_log (
    id           TEXT PRIMARY KEY,                  -- UUID
    user_id      TEXT REFERENCES users(id),         -- nullable: system/unauth events
    action       TEXT NOT NULL,                     -- auth.login, fs.write, container.start, ...
    target_type  TEXT,                              -- node|container|file|secret|...
    target_id    TEXT,
    detail_json  TEXT,                              -- structured before/after or summary
    result       TEXT NOT NULL,                     -- success|error
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_activity_created ON activity_log(created_at);
CREATE INDEX idx_activity_user    ON activity_log(user_id);
CREATE INDEX idx_activity_action  ON activity_log(action);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE activity_log;
DROP TABLE sessions;
DROP TABLE users;
-- +goose StatementEnd

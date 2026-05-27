-- +goose Up
-- +goose StatementBegin

-- bookmarks: per-user quick-access pointers to resources (Feature 24). Deleting
-- a user removes their bookmarks (CASCADE). order_index drives drag-to-reorder.
CREATE TABLE bookmarks (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label         TEXT NOT NULL,
    resource_type TEXT NOT NULL,            -- node | container | vm | path | file
    resource_ref  TEXT NOT NULL,            -- e.g. container id, or "<nodeID>:<path>"
    group_name    TEXT NOT NULL DEFAULT '',
    order_index   INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_bookmarks_user ON bookmarks(user_id, order_index);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE bookmarks;
-- +goose StatementEnd

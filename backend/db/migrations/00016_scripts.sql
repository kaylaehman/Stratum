-- +goose Up
-- +goose StatementBegin

-- scripts: saved shell scripts for the script runner (Feature 27). Run against
-- selected hosts over SSH (never against containers).
CREATE TABLE scripts (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE scripts;
-- +goose StatementEnd

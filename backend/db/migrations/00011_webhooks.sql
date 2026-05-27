-- +goose Up
-- +goose StatementBegin

-- webhook_configs: Slack/Discord notification targets (Feature 26). triggers is
-- a JSON array of trigger keys (e.g. ["port.new","container.crash"]).
CREATE TABLE webhook_configs (
    id        TEXT PRIMARY KEY,
    name      TEXT NOT NULL,
    url       TEXT NOT NULL,
    provider  TEXT NOT NULL,            -- slack | discord
    triggers  TEXT NOT NULL DEFAULT '[]',
    enabled   INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE webhook_configs;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
CREATE TABLE automations (
  key              TEXT PRIMARY KEY,
  enabled          INTEGER NOT NULL DEFAULT 0,
  interval_seconds INTEGER NOT NULL DEFAULT 3600,
  config_json      TEXT NOT NULL DEFAULT '{}',
  last_run         TIMESTAMP,
  last_status      TEXT NOT NULL DEFAULT '',
  last_detail      TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS automations;
-- +goose StatementEnd

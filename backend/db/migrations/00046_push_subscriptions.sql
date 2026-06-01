-- +goose Up
-- +goose StatementBegin
CREATE TABLE push_subscriptions (
  id          TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL,
  endpoint    TEXT NOT NULL UNIQUE,
  p256dh      TEXT NOT NULL,
  auth        TEXT NOT NULL,
  created_at  TEXT NOT NULL
);

CREATE INDEX push_subscriptions_user ON push_subscriptions (user_id);

CREATE TABLE push_vapid (
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  private_key TEXT NOT NULL,
  public_key  TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS push_subscriptions;
DROP TABLE IF EXISTS push_vapid;
-- +goose StatementEnd

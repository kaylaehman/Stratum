-- +goose Up
-- +goose StatementBegin
CREATE TABLE alert_policies (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL,
  enabled         INTEGER NOT NULL DEFAULT 1,
  min_severity    TEXT NOT NULL DEFAULT 'info',
  channels_json   TEXT NOT NULL DEFAULT '[]',
  match_json      TEXT NOT NULL DEFAULT '{}',
  quiet_hours_json TEXT,
  dedup_window_sec INTEGER NOT NULL DEFAULT 0,
  escalate_json   TEXT,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE TABLE alert_deliveries (
  id          TEXT PRIMARY KEY,
  policy_id   TEXT NOT NULL,
  alert_key   TEXT NOT NULL,
  severity    TEXT NOT NULL,
  channel     TEXT NOT NULL,
  status      TEXT NOT NULL,  -- delivered|suppressed_dedup|suppressed_quiet
  created_at  TEXT NOT NULL
);

CREATE INDEX alert_deliveries_key_time ON alert_deliveries (alert_key, created_at DESC);
CREATE INDEX alert_deliveries_policy   ON alert_deliveries (policy_id, created_at DESC);

-- Seed the default back-compat policy: route everything to all channels.
-- Channels is empty (will be resolved to all at routing time per back-compat semantics),
-- min_severity=info, no dedup, no quiet hours.
INSERT INTO alert_policies (id, name, enabled, min_severity, channels_json, match_json, quiet_hours_json, dedup_window_sec, escalate_json, created_at, updated_at)
VALUES (
  'default',
  'Default — route all alerts to all channels',
  1,
  'info',
  '[]',
  '{}',
  NULL,
  0,
  NULL,
  strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
  strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS alert_deliveries;
DROP TABLE IF EXISTS alert_policies;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- Migrate any existing cve_schedules rows into the scheduled_cve_scan
-- automation row. The automation stores its targets as a JSON config:
--   {"targets": [{"node_id": "...", "container_id": ""}]}
-- or {"targets": [{"node_id": "", "container_id": "..."}]}
-- An empty targets list means "scan everything" (preserved default behaviour).
--
-- Strategy:
--   1. Ensure the scheduled_cve_scan row exists in automations (it is
--      created by code on first use, but may not exist in older DBs).
--   2. Build a JSON array from cve_schedules rows and store it as config.
--   3. If any enabled schedule exists, set the automation to enabled.
--   4. Drop the cve_schedules table and its index.
--
-- This migration is idempotent: if cve_schedules is already empty or the
-- table does not exist, the INSERT/UPDATE is a no-op and the DROP succeeds.

-- 1. Ensure the automation row exists so we can update it.
INSERT OR IGNORE INTO automations (key, enabled, interval_seconds, config_json, updated_at)
VALUES (
    'scheduled_cve_scan',
    0,
    86400,
    '{"targets":[]}',
    CURRENT_TIMESTAMP
);

-- 2+3. Merge schedule targets into the automation config. We set enabled=1
--      when at least one enabled cve_schedule row existed, and build a
--      JSON array of target objects from all rows (enabled or not, so no
--      targets are silently lost).
UPDATE automations
SET
    config_json = COALESCE(
        (
            SELECT
                '{"targets":[' ||
                GROUP_CONCAT(
                    CASE target_type
                        WHEN 'node'      THEN '{"node_id":"' || target_id || '","container_id":""}'
                        WHEN 'container' THEN '{"node_id":"","container_id":"' || target_id || '"}'
                        ELSE                  '{"node_id":"","container_id":"' || target_id || '"}'
                    END,
                    ','
                ) ||
                ']}'
            FROM cve_schedules
        ),
        '{"targets":[]}'
    ),
    enabled = CASE
        WHEN EXISTS (SELECT 1 FROM cve_schedules WHERE enabled = 1) THEN 1
        ELSE enabled
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'scheduled_cve_scan';

-- 4. Remove the old table.
DROP INDEX IF EXISTS idx_cve_schedules_enabled;
DROP TABLE IF EXISTS cve_schedules;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Recreate the cve_schedules table (empty -- we cannot recover the migrated rows).
CREATE TABLE IF NOT EXISTS cve_schedules (
    id               TEXT PRIMARY KEY,
    target_type      TEXT NOT NULL DEFAULT 'node',
    target_id        TEXT NOT NULL,
    label            TEXT NOT NULL DEFAULT '',
    interval_seconds INTEGER NOT NULL DEFAULT 86400,
    enabled          INTEGER NOT NULL DEFAULT 1,
    created_by       TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_run_at      TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cve_schedules_enabled ON cve_schedules (enabled);

-- +goose StatementEnd

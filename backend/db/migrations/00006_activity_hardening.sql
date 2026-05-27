-- +goose Up
-- Append-only enforcement + filter/keyset indexes for the activity_log table
-- (owned by 00001). The triggers make append-only STRUCTURAL: even a stray query
-- or a SQL-injected statement cannot UPDATE or DELETE an audit row.
--
-- Threat model: these triggers stop ACCIDENTAL or INJECTED mutation only. The
-- app connects with full rights, so a fully compromised backend process could
-- DROP TRIGGER then delete, and direct access to the .db file at rest bypasses
-- SQLite entirely. Cryptographic tamper-evidence (a hash chain) is the future
-- answer; the trigger is the MVP guarantee.
--
-- FUTURE-MIGRATION NOTE: any later migration that must alter activity_log (add a
-- column, etc.) must DROP these triggers, run the DDL, then recreate them —
-- otherwise the DDL's internal writes can ABORT against the trigger.

-- +goose StatementBegin
CREATE TRIGGER activity_log_no_update
BEFORE UPDATE ON activity_log
BEGIN
    SELECT RAISE(ABORT, 'activity_log is append-only');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER activity_log_no_delete
BEFORE DELETE ON activity_log
BEGIN
    SELECT RAISE(ABORT, 'activity_log is append-only');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_activity_target ON activity_log(target_type, target_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_activity_result ON activity_log(result);
-- +goose StatementEnd

-- Keyset pagination driver: created_at DESC range scan; rowid (the implicit
-- monotonic btree key) is the tiebreaker, so no extra index column is needed.
-- +goose StatementBegin
CREATE INDEX idx_activity_keyset ON activity_log(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- Drops the triggers + indexes only. NEVER deletes audit data (the table itself
-- is owned by 00001's down migration).
-- +goose StatementBegin
DROP TRIGGER IF EXISTS activity_log_no_update;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS activity_log_no_delete;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_activity_target;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_activity_result;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_activity_keyset;
-- +goose StatementEnd

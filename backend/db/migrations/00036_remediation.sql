-- +goose Up
-- +goose StatementBegin

-- remediation_proposals: agentic-remediation proposals generated from diagnostic
-- results, runbooks, or AI suggestions. A proposal is immutable once created
-- (generate → approve/reject → execute); each status transition is audited.
--
-- source: diagnostic | runbook | ai
-- risk_level: low | medium | high | destructive
-- status: proposed | approved | rejected | executed | failed
--
-- node_id / container_id: at least one must be non-null (the execution target).
-- commands_json: JSON array of strings — the exact shell commands to run.
-- approved_by: user id of the approver; NULL until approved.
-- stdout / stderr / exit_code: captured on execution; NULL until executed.
CREATE TABLE remediation_proposals (
    id            TEXT PRIMARY KEY,
    source        TEXT NOT NULL DEFAULT 'ai',
    title         TEXT NOT NULL,
    rationale     TEXT NOT NULL DEFAULT '',
    node_id       TEXT,
    container_id  TEXT,
    commands_json TEXT NOT NULL DEFAULT '[]',
    risk_level    TEXT NOT NULL DEFAULT 'medium',
    status        TEXT NOT NULL DEFAULT 'proposed',
    created_by    TEXT NOT NULL,
    approved_by   TEXT,
    stdout        TEXT,
    stderr        TEXT,
    exit_code     INTEGER,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    approved_at   TIMESTAMP,
    executed_at   TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_remediation_node    ON remediation_proposals (node_id);
CREATE INDEX IF NOT EXISTS idx_remediation_status  ON remediation_proposals (status);
CREATE INDEX IF NOT EXISTS idx_remediation_creator ON remediation_proposals (created_by);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS remediation_proposals;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- runbooks: saved diagnostic/remediation procedures the AI assistant can
-- reference (Feature F9). trigger_conditions and steps are JSON arrays of
-- strings. requires_approval marks procedures whose steps must not be run
-- without explicit confirmation (the execution engine is a follow-on; for now
-- runbooks are reference material surfaced to the assistant).
CREATE TABLE runbooks (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    description        TEXT NOT NULL DEFAULT '',
    trigger_conditions TEXT NOT NULL DEFAULT '[]',
    steps              TEXT NOT NULL DEFAULT '[]',
    requires_approval  INTEGER NOT NULL DEFAULT 1,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE runbooks;
-- +goose StatementEnd

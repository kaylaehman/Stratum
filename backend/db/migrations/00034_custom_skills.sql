-- +goose Up
-- +goose StatementBegin

-- custom_skills: user-authored container-troubleshooting skills (the editable
-- counterpart to the read-only built-in library shipped under assets/skills/).
-- The full YAML is stored verbatim so it round-trips through the editor exactly
-- as written; id is the skill's own id (parsed from the YAML) and is unique so a
-- custom skill can be upserted. On startup these are parsed and merged into the
-- in-memory skill library alongside the built-in skills.
CREATE TABLE custom_skills (
    id         TEXT PRIMARY KEY,
    yaml       TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE custom_skills;
-- +goose StatementEnd

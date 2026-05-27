-- +goose Up
-- +goose StatementBegin

-- templates: saved/versioned Docker Compose stacks (Feature 14). variables is a
-- JSON array of {name,description,default}; tags is a JSON array of strings.
CREATE TABLE templates (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    tags         TEXT NOT NULL DEFAULT '[]',
    compose_yaml TEXT NOT NULL,
    variables    TEXT NOT NULL DEFAULT '[]',
    version      INTEGER NOT NULL DEFAULT 1,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- template_versions: an immutable snapshot per saved version (for diff/history).
CREATE TABLE template_versions (
    id           TEXT PRIMARY KEY,
    template_id  TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    version      INTEGER NOT NULL,
    compose_yaml TEXT NOT NULL,
    variables    TEXT NOT NULL DEFAULT '[]',
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_template_versions ON template_versions(template_id, version);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE template_versions;
DROP TABLE templates;
-- +goose StatementEnd

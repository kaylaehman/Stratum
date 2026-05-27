-- +goose Up
-- +goose StatementBegin

-- image_scans: one row per scanned image digest with severity tallies (Feature
-- 20). Cached so an unchanged digest isn't re-scanned unless forced.
CREATE TABLE image_scans (
    image_digest TEXT PRIMARY KEY,
    image        TEXT NOT NULL,
    scanned_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    critical     INTEGER NOT NULL DEFAULT 0,
    high         INTEGER NOT NULL DEFAULT 0,
    medium       INTEGER NOT NULL DEFAULT 0,
    low          INTEGER NOT NULL DEFAULT 0,
    unknown      INTEGER NOT NULL DEFAULT 0
);

-- cve_results: per-vulnerability detail for a scanned digest.
CREATE TABLE cve_results (
    id                TEXT PRIMARY KEY,
    image_digest      TEXT NOT NULL,
    cve_id            TEXT NOT NULL,
    severity          TEXT NOT NULL,
    package           TEXT NOT NULL,
    installed_version TEXT NOT NULL DEFAULT '',
    fixed_version     TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_cve_results_digest ON cve_results(image_digest);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE cve_results;
DROP TABLE image_scans;
-- +goose StatementEnd

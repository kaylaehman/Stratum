-- +goose Up
-- +goose StatementBegin

-- Remove stale CVE scan records that were stored with an empty image_digest.
-- These were written before the resolveDigest() fix (commit 956ef4c) which
-- ensured the digest key is never the empty string. The empty-keyed rows are
-- unreachable from the detail endpoint (the frontend's useCVEDetail hook guards
-- on digest.length > 0 and disables the query for empty digests), so they
-- permanently show non-zero severity counts with an empty vulnerability list.
-- Deleting them forces a fresh re-scan under the correct digest key.
DELETE FROM cve_results WHERE image_digest = '';
DELETE FROM image_scans  WHERE image_digest = '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Orphaned rows are not worth restoring; no-op rollback.
-- +goose StatementEnd

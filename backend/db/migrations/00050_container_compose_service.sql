-- +goose Up
-- +goose StatementBegin
-- The Docker Compose SERVICE name (label com.docker.compose.service) — the alias
-- reverse-proxy/tunnel ingress rules target (e.g. http://jellyfin:8096), distinct
-- from the container's full name (project-jellyfin-1). Lets proxy rules resolve to
-- the container instead of only the host.
ALTER TABLE containers ADD COLUMN compose_service TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE containers DROP COLUMN compose_service;
-- +goose StatementEnd

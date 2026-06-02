-- +goose Up
-- +goose StatementBegin
-- config_json holds NON-SECRET per-node reverse-proxy provider config: a
-- provider-kind override plus provider-specific fields (e.g. Cloudflare
-- account_id / tunnel_id). The provider is normally auto-detected from the
-- node's container images; a stored kind here lets a node opt into an API-based
-- provider that cannot be inferred from images (e.g. "cloudflare-api" for a
-- dashboard-managed cloudflared tunnel). The API token continues to live in
-- token_encrypted (AES-sealed); secrets are never stored here.
ALTER TABLE proxy_config ADD COLUMN config_json TEXT NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE proxy_config DROP COLUMN config_json;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
-- Claude OAuth ("claude.ai -p method", Feature 31): tokens obtained via the
-- PKCE sign-in flow are AES-sealed at rest, exactly like api_key_encrypted.
-- oauth_expires_at drives proactive refresh. All nullable: only populated when
-- the operator connects via OAuth.
ALTER TABLE ai_config ADD COLUMN oauth_access_encrypted BLOB;
ALTER TABLE ai_config ADD COLUMN oauth_refresh_encrypted BLOB;
ALTER TABLE ai_config ADD COLUMN oauth_expires_at TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai_config DROP COLUMN oauth_access_encrypted;
ALTER TABLE ai_config DROP COLUMN oauth_refresh_encrypted;
ALTER TABLE ai_config DROP COLUMN oauth_expires_at;
-- +goose StatementEnd

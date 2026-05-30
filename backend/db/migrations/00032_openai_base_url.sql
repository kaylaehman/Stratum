-- +goose Up
-- +goose StatementBegin
-- Optional base URL for the OpenAI provider, so it can target any
-- OpenAI-compatible endpoint (claude-max-api-proxy, LiteLLM, vLLM, OpenRouter,
-- …). Empty => the real OpenAI API.
ALTER TABLE ai_config ADD COLUMN openai_base_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai_config DROP COLUMN openai_base_url;
-- +goose StatementEnd

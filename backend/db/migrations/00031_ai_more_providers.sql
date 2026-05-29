-- +goose Up
-- +goose StatementBegin
-- OpenAI and Gemini API-key providers (Feature 31 extension). Each keeps its own
-- model name; the API key reuses the shared api_key_encrypted column (one active
-- API-key provider at a time).
ALTER TABLE ai_config ADD COLUMN openai_model TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_config ADD COLUMN gemini_model TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai_config DROP COLUMN openai_model;
ALTER TABLE ai_config DROP COLUMN gemini_model;
-- +goose StatementEnd

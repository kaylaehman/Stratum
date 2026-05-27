package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// GetAIConfig returns the single ai_config row, or ErrNotFound when unset.
func (s *Store) GetAIConfig(ctx context.Context) (appdb.AIConfig, error) {
	var c appdb.AIConfig
	var key []byte
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT provider, ollama_base_url, ollama_model, claude_model, api_key_encrypted, updated_at
		 FROM ai_config WHERE id = 1`).
		Scan(&c.Provider, &c.OllamaBaseURL, &c.OllamaModel, &c.ClaudeModel, &key, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.AIConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.AIConfig{}, fmt.Errorf("sqlite: get ai_config: %w", err)
	}
	c.APIKeyEncrypted = key
	c.UpdatedAt, _ = parseTS(updatedAt)
	return c, nil
}

// UpsertAIConfig writes the single ai_config row.
func (s *Store) UpsertAIConfig(ctx context.Context, c appdb.AIConfig) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ai_config (id, provider, ollama_base_url, ollama_model, claude_model, api_key_encrypted, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   provider=excluded.provider,
		   ollama_base_url=excluded.ollama_base_url,
		   ollama_model=excluded.ollama_model,
		   claude_model=excluded.claude_model,
		   api_key_encrypted=excluded.api_key_encrypted,
		   updated_at=excluded.updated_at`,
		c.Provider, c.OllamaBaseURL, c.OllamaModel, c.ClaudeModel, c.APIKeyEncrypted, tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: upsert ai_config: %w", err)
	}
	return nil
}

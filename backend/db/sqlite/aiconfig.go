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
	var key, oauthAccess, oauthRefresh []byte
	var updatedAt string
	var oauthExpires sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT provider, ollama_base_url, ollama_model, claude_model, openai_model, openai_base_url, gemini_model, api_key_encrypted,
		        oauth_access_encrypted, oauth_refresh_encrypted, oauth_expires_at, updated_at
		 FROM ai_config WHERE id = 1`).
		Scan(&c.Provider, &c.OllamaBaseURL, &c.OllamaModel, &c.ClaudeModel, &c.OpenAIModel, &c.OpenAIBaseURL, &c.GeminiModel, &key,
			&oauthAccess, &oauthRefresh, &oauthExpires, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.AIConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.AIConfig{}, fmt.Errorf("sqlite: get ai_config: %w", err)
	}
	c.APIKeyEncrypted = key
	c.OAuthAccessEncrypted = oauthAccess
	c.OAuthRefreshEncrypted = oauthRefresh
	if oauthExpires.Valid {
		c.OAuthExpiresAt, _ = parseTS(oauthExpires.String)
	}
	c.UpdatedAt, _ = parseTS(updatedAt)
	return c, nil
}

// UpsertAIConfig writes the single ai_config row.
func (s *Store) UpsertAIConfig(ctx context.Context, c appdb.AIConfig) error {
	var oauthExpires any
	if !c.OAuthExpiresAt.IsZero() {
		oauthExpires = tsText(c.OAuthExpiresAt)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ai_config (id, provider, ollama_base_url, ollama_model, claude_model, openai_model, openai_base_url, gemini_model, api_key_encrypted,
		                        oauth_access_encrypted, oauth_refresh_encrypted, oauth_expires_at, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   provider=excluded.provider,
		   ollama_base_url=excluded.ollama_base_url,
		   ollama_model=excluded.ollama_model,
		   claude_model=excluded.claude_model,
		   openai_model=excluded.openai_model,
		   openai_base_url=excluded.openai_base_url,
		   gemini_model=excluded.gemini_model,
		   api_key_encrypted=excluded.api_key_encrypted,
		   oauth_access_encrypted=excluded.oauth_access_encrypted,
		   oauth_refresh_encrypted=excluded.oauth_refresh_encrypted,
		   oauth_expires_at=excluded.oauth_expires_at,
		   updated_at=excluded.updated_at`,
		c.Provider, c.OllamaBaseURL, c.OllamaModel, c.ClaudeModel, c.OpenAIModel, c.OpenAIBaseURL, c.GeminiModel, c.APIKeyEncrypted,
		c.OAuthAccessEncrypted, c.OAuthRefreshEncrypted, oauthExpires, tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: upsert ai_config: %w", err)
	}
	return nil
}

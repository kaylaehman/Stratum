package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

func (s *Store) GetChatConfig(ctx context.Context) (appdb.ChatConfig, error) {
	var c appdb.ChatConfig
	var token []byte
	var allowed, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT provider, token_encrypted, allowed_chats, updated_at FROM chat_config WHERE id = 1`).
		Scan(&c.Provider, &token, &allowed, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.ChatConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.ChatConfig{}, fmt.Errorf("sqlite: get chat_config: %w", err)
	}
	c.TokenEncrypted = token
	_ = json.Unmarshal([]byte(allowed), &c.AllowedChats)
	if c.AllowedChats == nil {
		c.AllowedChats = []int64{}
	}
	c.UpdatedAt, _ = parseTS(updatedAt)
	return c, nil
}

func (s *Store) UpsertChatConfig(ctx context.Context, c appdb.ChatConfig) error {
	allowed, _ := json.Marshal(c.AllowedChats)
	if c.Provider == "" {
		c.Provider = "telegram"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chat_config (id, provider, token_encrypted, allowed_chats, updated_at)
		 VALUES (1, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   provider=excluded.provider,
		   token_encrypted=excluded.token_encrypted,
		   allowed_chats=excluded.allowed_chats,
		   updated_at=excluded.updated_at`,
		c.Provider, c.TokenEncrypted, string(allowed), tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: upsert chat_config: %w", err)
	}
	return nil
}

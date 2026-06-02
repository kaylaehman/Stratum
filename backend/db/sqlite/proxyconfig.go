package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func (s *Store) GetProxyConfig(ctx context.Context, nodeID string) (appdb.ProxyConfig, error) {
	var c appdb.ProxyConfig
	var token []byte
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT node_id, endpoint, token_encrypted, config_json, updated_at FROM proxy_config WHERE node_id = ?`, nodeID).
		Scan(&c.NodeID, &c.Endpoint, &token, &c.ConfigJSON, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.ProxyConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.ProxyConfig{}, fmt.Errorf("sqlite: get proxy_config: %w", err)
	}
	c.TokenEncrypted = token
	if c.ConfigJSON == "" {
		c.ConfigJSON = "{}"
	}
	c.UpdatedAt, _ = parseTS(updatedAt)
	return c, nil
}

func (s *Store) UpsertProxyConfig(ctx context.Context, c appdb.ProxyConfig) error {
	configJSON := c.ConfigJSON
	if configJSON == "" {
		configJSON = "{}"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO proxy_config (node_id, endpoint, token_encrypted, config_json, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(node_id) DO UPDATE SET
		   endpoint=excluded.endpoint,
		   token_encrypted=excluded.token_encrypted,
		   config_json=excluded.config_json,
		   updated_at=excluded.updated_at`,
		c.NodeID, c.Endpoint, c.TokenEncrypted, configJSON, tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: upsert proxy_config: %w", err)
	}
	return nil
}

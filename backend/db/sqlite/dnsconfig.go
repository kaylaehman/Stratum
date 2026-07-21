package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

func (s *Store) GetDNSConfig(ctx context.Context, nodeID string) (appdb.DNSConfig, error) {
	var c appdb.DNSConfig
	var token []byte
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT node_id, endpoint, token_encrypted, updated_at FROM dns_config WHERE node_id = ?`, nodeID).
		Scan(&c.NodeID, &c.Endpoint, &token, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.DNSConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.DNSConfig{}, fmt.Errorf("sqlite: get dns_config: %w", err)
	}
	c.TokenEncrypted = token
	c.UpdatedAt, _ = parseTS(updatedAt)
	return c, nil
}

func (s *Store) UpsertDNSConfig(ctx context.Context, c appdb.DNSConfig) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dns_config (node_id, endpoint, token_encrypted, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(node_id) DO UPDATE SET
		   endpoint=excluded.endpoint,
		   token_encrypted=excluded.token_encrypted,
		   updated_at=excluded.updated_at`,
		c.NodeID, c.Endpoint, c.TokenEncrypted, tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: upsert dns_config: %w", err)
	}
	return nil
}

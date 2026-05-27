package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func (s *Store) UpsertWOLConfig(ctx context.Context, c appdb.WOLConfig) error {
	if c.Broadcast == "" {
		c.Broadcast = "255.255.255.255"
	}
	if c.Port == 0 {
		c.Port = 9
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wol_config (node_id, mac, broadcast, port) VALUES (?, ?, ?, ?)
		 ON CONFLICT(node_id) DO UPDATE SET mac=excluded.mac, broadcast=excluded.broadcast, port=excluded.port`,
		c.NodeID, c.MAC, c.Broadcast, c.Port)
	if err != nil {
		return fmt.Errorf("sqlite: upsert wol_config: %w", err)
	}
	return nil
}

func (s *Store) GetWOLConfig(ctx context.Context, nodeID string) (appdb.WOLConfig, error) {
	var c appdb.WOLConfig
	err := s.db.QueryRowContext(ctx,
		`SELECT node_id, mac, broadcast, port FROM wol_config WHERE node_id = ?`, nodeID).
		Scan(&c.NodeID, &c.MAC, &c.Broadcast, &c.Port)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.WOLConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.WOLConfig{}, fmt.Errorf("sqlite: get wol_config: %w", err)
	}
	return c, nil
}

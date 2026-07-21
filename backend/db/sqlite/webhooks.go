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

const webhookColumns = `id, name, url, provider, triggers, enabled, created_at`

func (s *Store) CreateWebhook(ctx context.Context, c appdb.WebhookConfig) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_configs (`+webhookColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.URL, c.Provider, marshalStrings(c.Triggers), boolToInt(c.Enabled), tsText(c.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create webhook: %w", err)
	}
	return nil
}

func scanWebhook(sc rowScanner) (appdb.WebhookConfig, error) {
	var c appdb.WebhookConfig
	var triggers, createdAt string
	var enabled int
	if err := sc.Scan(&c.ID, &c.Name, &c.URL, &c.Provider, &triggers, &enabled, &createdAt); err != nil {
		return appdb.WebhookConfig{}, err
	}
	c.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(triggers), &c.Triggers)
	c.CreatedAt, _ = parseTS(createdAt)
	return c, nil
}

func (s *Store) ListWebhooks(ctx context.Context) ([]appdb.WebhookConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+webhookColumns+` FROM webhook_configs ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list webhooks: %w", err)
	}
	defer rows.Close()
	var out []appdb.WebhookConfig
	for rows.Next() {
		c, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetWebhook(ctx context.Context, id string) (appdb.WebhookConfig, error) {
	c, err := scanWebhook(s.db.QueryRowContext(ctx, `SELECT `+webhookColumns+` FROM webhook_configs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.WebhookConfig{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.WebhookConfig{}, fmt.Errorf("sqlite: get webhook: %w", err)
	}
	return c, nil
}

func (s *Store) UpdateWebhook(ctx context.Context, c appdb.WebhookConfig) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE webhook_configs SET name=?, url=?, provider=?, triggers=?, enabled=? WHERE id=?`,
		c.Name, c.URL, c.Provider, marshalStrings(c.Triggers), boolToInt(c.Enabled), c.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update webhook: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhook_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete webhook: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

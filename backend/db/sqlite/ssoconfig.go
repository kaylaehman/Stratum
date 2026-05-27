package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const ssoColumns = `id, node_id, container_name, enabled, method, provider_url, client_id, client_secret_encrypted, allowed_groups, session_duration_secs, updated_at`

func (s *Store) ListSSOConfigs(ctx context.Context) ([]appdb.SSOConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+ssoColumns+` FROM sso_config ORDER BY node_id, container_name`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list sso_config: %w", err)
	}
	defer rows.Close()
	var out []appdb.SSOConfig
	for rows.Next() {
		c, err := scanSSO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertSSOConfig writes a config keyed by (node_id, container_name) and returns
// the stored row (with its canonical id, whether inserted or updated).
func (s *Store) UpsertSSOConfig(ctx context.Context, c appdb.SSOConfig) (appdb.SSOConfig, error) {
	groups, _ := json.Marshal(c.AllowedGroups)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sso_config (`+ssoColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(node_id, container_name) DO UPDATE SET
		   enabled=excluded.enabled, method=excluded.method, provider_url=excluded.provider_url,
		   client_id=excluded.client_id, client_secret_encrypted=excluded.client_secret_encrypted,
		   allowed_groups=excluded.allowed_groups, session_duration_secs=excluded.session_duration_secs,
		   updated_at=excluded.updated_at`,
		c.ID, c.NodeID, c.ContainerName, boolToInt(c.Enabled), c.Method, c.ProviderURL, c.ClientID,
		c.ClientSecretEncrypted, string(groups), c.SessionDurationSecs, tsText(time.Now()))
	if err != nil {
		return appdb.SSOConfig{}, fmt.Errorf("sqlite: upsert sso_config: %w", err)
	}
	return scanSSO(s.db.QueryRowContext(ctx,
		`SELECT `+ssoColumns+` FROM sso_config WHERE node_id = ? AND container_name = ?`, c.NodeID, c.ContainerName))
}

func (s *Store) DeleteSSOConfig(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sso_config WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete sso_config: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func scanSSO(sc rowScanner) (appdb.SSOConfig, error) {
	var c appdb.SSOConfig
	var enabled int
	var secret []byte
	var groups, updatedAt string
	if err := sc.Scan(&c.ID, &c.NodeID, &c.ContainerName, &enabled, &c.Method, &c.ProviderURL,
		&c.ClientID, &secret, &groups, &c.SessionDurationSecs, &updatedAt); err != nil {
		return appdb.SSOConfig{}, err
	}
	c.Enabled = enabled != 0
	c.ClientSecretEncrypted = secret
	c.HasClientSecret = len(secret) > 0
	_ = json.Unmarshal([]byte(groups), &c.AllowedGroups)
	if c.AllowedGroups == nil {
		c.AllowedGroups = []string{}
	}
	c.UpdatedAt, _ = parseTS(updatedAt)
	return c, nil
}

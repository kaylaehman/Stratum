package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const certColumns = `id, node_id, source, domain, sans, issuer, path, not_before, not_after, last_checked`

// ReplaceCertsByNode swaps a node's cert rows for the freshly-scanned set, in a
// transaction so a reader never sees a half-replaced set.
func (s *Store) ReplaceCertsByNode(ctx context.Context, nodeID string, certs []appdb.CertInfo) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin certs tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM certs WHERE node_id = ?`, nodeID); err != nil {
		return fmt.Errorf("sqlite: clear certs: %w", err)
	}
	for _, c := range certs {
		sans, _ := json.Marshal(c.SANs)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO certs (`+certColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, nodeID, c.Source, c.Domain, string(sans), c.Issuer, c.Path,
			nullTSText(c.NotBefore), nullTSText(c.NotAfter), tsText(c.LastChecked)); err != nil {
			return fmt.Errorf("sqlite: insert cert: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ListCerts(ctx context.Context) ([]appdb.CertInfo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+certColumns+` FROM certs ORDER BY not_after`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list certs: %w", err)
	}
	defer rows.Close()
	var out []appdb.CertInfo
	for rows.Next() {
		var c appdb.CertInfo
		var sans string
		var notBefore, notAfter, lastChecked sql.NullString
		if err := rows.Scan(&c.ID, &c.NodeID, &c.Source, &c.Domain, &sans, &c.Issuer, &c.Path,
			&notBefore, &notAfter, &lastChecked); err != nil {
			return nil, fmt.Errorf("sqlite: scan cert: %w", err)
		}
		_ = json.Unmarshal([]byte(sans), &c.SANs)
		if c.SANs == nil {
			c.SANs = []string{}
		}
		if c.NotBefore, err = scanNullTS(notBefore); err != nil {
			return nil, err
		}
		if c.NotAfter, err = scanNullTS(notAfter); err != nil {
			return nil, err
		}
		c.LastChecked, _ = scanTS(lastChecked)
		out = append(out, c)
	}
	return out, rows.Err()
}

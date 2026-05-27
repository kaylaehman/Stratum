package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const imageUpdateColumns = `container_id, node_id, image, status, current_digest, latest_digest, checked_at`

func (s *Store) UpsertImageUpdate(ctx context.Context, r appdb.ImageUpdateRow) error {
	if r.CheckedAt.IsZero() {
		r.CheckedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO image_updates (`+imageUpdateColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(container_id) DO UPDATE SET
		   node_id=excluded.node_id, image=excluded.image, status=excluded.status,
		   current_digest=excluded.current_digest, latest_digest=excluded.latest_digest,
		   checked_at=excluded.checked_at`,
		r.ContainerID, r.NodeID, r.Image, r.Status, r.CurrentDigest, r.LatestDigest, tsText(r.CheckedAt))
	if err != nil {
		return fmt.Errorf("sqlite: upsert image_update: %w", err)
	}
	return nil
}

func (s *Store) ListImageUpdates(ctx context.Context) ([]appdb.ImageUpdateRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+imageUpdateColumns+` FROM image_updates`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list image_updates: %w", err)
	}
	defer rows.Close()
	var out []appdb.ImageUpdateRow
	for rows.Next() {
		var r appdb.ImageUpdateRow
		var checkedAt string
		if err := rows.Scan(&r.ContainerID, &r.NodeID, &r.Image, &r.Status, &r.CurrentDigest, &r.LatestDigest, &checkedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan image_update: %w", err)
		}
		r.CheckedAt, _ = parseTS(checkedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

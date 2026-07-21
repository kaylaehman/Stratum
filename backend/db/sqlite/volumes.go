package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

func (s *Store) InsertVolumeSample(ctx context.Context, v appdb.VolumeSample) error {
	if v.SampledAt.IsZero() {
		v.SampledAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO volume_samples (id, node_id, volume_name, size_bytes, ref_count, sampled_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		v.ID, v.NodeID, v.VolumeName, v.SizeBytes, v.RefCount, tsText(v.SampledAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert volume sample: %w", err)
	}
	return nil
}

// PruneVolumeSamplesBefore deletes size-trend samples older than cutoff, bounding
// growth of the (non-audit, regenerable) volume_samples table. Returns the number
// of rows removed.
func (s *Store) PruneVolumeSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM volume_samples WHERE sampled_at < ?`, tsText(cutoff))
	if err != nil {
		return 0, fmt.Errorf("sqlite: prune volume samples: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) ListVolumeSamplesByNode(ctx context.Context, nodeID string) ([]appdb.VolumeSample, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, node_id, volume_name, size_bytes, ref_count, sampled_at
		 FROM volume_samples WHERE node_id = ? ORDER BY sampled_at ASC`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list volume samples: %w", err)
	}
	defer rows.Close()
	var out []appdb.VolumeSample
	for rows.Next() {
		var v appdb.VolumeSample
		var sampledAt string
		if err := rows.Scan(&v.ID, &v.NodeID, &v.VolumeName, &v.SizeBytes, &v.RefCount, &sampledAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan volume sample: %w", err)
		}
		v.SampledAt, _ = parseTS(sampledAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

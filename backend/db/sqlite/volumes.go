package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
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

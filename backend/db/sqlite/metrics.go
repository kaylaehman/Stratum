package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

func (s *Store) InsertResourceSample(ctx context.Context, r appdb.ResourceSample) error {
	if r.SampledAt.IsZero() {
		r.SampledAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO resource_samples
		   (id, container_id, node_id, cpu_pct, mem_bytes, mem_limit_bytes, disk_read_bytes, disk_write_bytes, sampled_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ContainerID, r.NodeID, r.CPUPct, r.MemBytes, r.MemLimitBytes,
		r.DiskReadBytes, r.DiskWriteBytes, tsText(r.SampledAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert resource sample: %w", err)
	}
	return nil
}

func (s *Store) ListResourceSamples(ctx context.Context, containerID string, from, to time.Time) ([]appdb.ResourceSample, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, container_id, node_id, cpu_pct, mem_bytes, mem_limit_bytes, disk_read_bytes, disk_write_bytes, sampled_at
		 FROM resource_samples
		 WHERE container_id = ? AND sampled_at >= ? AND sampled_at <= ?
		 ORDER BY sampled_at ASC`,
		containerID, tsText(from), tsText(to))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list resource samples: %w", err)
	}
	defer rows.Close()
	var out []appdb.ResourceSample
	for rows.Next() {
		var r appdb.ResourceSample
		var sampledAt string
		if err := rows.Scan(&r.ID, &r.ContainerID, &r.NodeID, &r.CPUPct, &r.MemBytes,
			&r.MemLimitBytes, &r.DiskReadBytes, &r.DiskWriteBytes, &sampledAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan resource sample: %w", err)
		}
		r.SampledAt, _ = parseTS(sampledAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) PruneResourceSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM resource_samples WHERE sampled_at < ?`, tsText(cutoff))
	if err != nil {
		return 0, fmt.Errorf("sqlite: prune resource samples: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

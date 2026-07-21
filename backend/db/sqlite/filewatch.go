package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

func (s *Store) CreateFileWatch(ctx context.Context, w appdb.FileWatch) error {
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_watches (id, node_id, path, recursive, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		w.ID, w.NodeID, w.Path, boolToInt(w.Recursive), w.CreatedBy, tsText(w.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create file_watch: %w", err)
	}
	return nil
}

func (s *Store) ListFileWatchesByNode(ctx context.Context, nodeID string) ([]appdb.FileWatch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, node_id, path, recursive, created_by, created_at FROM file_watches WHERE node_id = ? ORDER BY path`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list file_watches: %w", err)
	}
	defer rows.Close()
	var out []appdb.FileWatch
	for rows.Next() {
		var w appdb.FileWatch
		var recursive int
		var createdAt string
		if err := rows.Scan(&w.ID, &w.NodeID, &w.Path, &recursive, &w.CreatedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan file_watch: %w", err)
		}
		w.Recursive = recursive != 0
		w.CreatedAt, _ = parseTS(createdAt)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) DeleteFileWatch(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM file_watches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete file_watch: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) InsertFileEvent(ctx context.Context, e appdb.FileEvent) error {
	if e.DetectedAt.IsZero() {
		e.DetectedAt = time.Now()
	}
	if e.EventType == "" {
		e.EventType = "modified"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_events (id, node_id, path, event_type, detected_at) VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.NodeID, e.Path, e.EventType, tsText(e.DetectedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert file_event: %w", err)
	}
	return nil
}

func (s *Store) ListFileEvents(ctx context.Context, nodeID string, limit int) ([]appdb.FileEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var (
		rows *sql.Rows
		err  error
	)
	if nodeID == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, node_id, path, event_type, detected_at FROM file_events ORDER BY detected_at DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, node_id, path, event_type, detected_at FROM file_events WHERE node_id = ? ORDER BY detected_at DESC LIMIT ?`, nodeID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: list file_events: %w", err)
	}
	defer rows.Close()
	var out []appdb.FileEvent
	for rows.Next() {
		var e appdb.FileEvent
		var detectedAt string
		if err := rows.Scan(&e.ID, &e.NodeID, &e.Path, &e.EventType, &detectedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan file_event: %w", err)
		}
		e.DetectedAt, _ = parseTS(detectedAt)
		out = append(out, e)
	}
	return out, rows.Err()
}

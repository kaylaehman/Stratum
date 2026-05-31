package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// --- uptime_monitors ---

func (s *Store) CreateUptimeMonitor(ctx context.Context, m appdb.UptimeMonitor) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = m.CreatedAt
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO uptime_monitors
		   (id, name, type, target, interval_seconds, timeout_ms, expected, enabled, node_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Type, m.Target, m.IntervalSeconds, m.TimeoutMs,
		m.Expected, boolToInt(m.Enabled), nullStr(m.NodeID),
		tsText(m.CreatedAt), tsText(m.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create uptime monitor: %w", err)
	}
	return nil
}

func (s *Store) GetUptimeMonitor(ctx context.Context, id string) (appdb.UptimeMonitor, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, type, target, interval_seconds, timeout_ms, expected, enabled, node_id, created_at, updated_at
		 FROM uptime_monitors WHERE id = ?`, id)
	m, err := scanUptimeMonitor(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.UptimeMonitor{}, appdb.ErrNotFound
	}
	return m, err
}

func (s *Store) ListUptimeMonitors(ctx context.Context) ([]appdb.UptimeMonitor, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, target, interval_seconds, timeout_ms, expected, enabled, node_id, created_at, updated_at
		 FROM uptime_monitors ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list uptime monitors: %w", err)
	}
	defer rows.Close()
	var out []appdb.UptimeMonitor
	for rows.Next() {
		m, err := scanUptimeMonitor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUptimeMonitor(ctx context.Context, m appdb.UptimeMonitor) error {
	m.UpdatedAt = time.Now()
	res, err := s.db.ExecContext(ctx,
		`UPDATE uptime_monitors
		 SET name=?, type=?, target=?, interval_seconds=?, timeout_ms=?,
		     expected=?, enabled=?, node_id=?, updated_at=?
		 WHERE id=?`,
		m.Name, m.Type, m.Target, m.IntervalSeconds, m.TimeoutMs,
		m.Expected, boolToInt(m.Enabled), nullStr(m.NodeID),
		tsText(m.UpdatedAt), m.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update uptime monitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUptimeMonitor(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM uptime_monitors WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete uptime monitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// scanUptimeMonitor works for both *sql.Row and *sql.Rows via the rowScanner interface.
func scanUptimeMonitor(sc rowScanner) (appdb.UptimeMonitor, error) {
	var m appdb.UptimeMonitor
	var nodeID sql.NullString
	var createdAt, updatedAt sql.NullString
	var enabledInt int
	if err := sc.Scan(
		&m.ID, &m.Name, &m.Type, &m.Target,
		&m.IntervalSeconds, &m.TimeoutMs, &m.Expected,
		&enabledInt, &nodeID, &createdAt, &updatedAt,
	); err != nil {
		return appdb.UptimeMonitor{}, fmt.Errorf("sqlite: scan uptime monitor: %w", err)
	}
	m.Enabled = enabledInt != 0
	m.NodeID = ptrFromNull(nodeID)
	var err error
	if m.CreatedAt, err = scanTS(createdAt); err != nil {
		return appdb.UptimeMonitor{}, err
	}
	if m.UpdatedAt, err = scanTS(updatedAt); err != nil {
		return appdb.UptimeMonitor{}, err
	}
	return m, nil
}

// --- uptime_results ---

func (s *Store) InsertUptimeResult(ctx context.Context, r appdb.UptimeResult) error {
	if r.CheckedAt.IsZero() {
		r.CheckedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO uptime_results (id, monitor_id, checked_at, status, response_time_ms, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.MonitorID, tsText(r.CheckedAt), r.Status, r.ResponseTimeMs, r.Error)
	if err != nil {
		return fmt.Errorf("sqlite: insert uptime result: %w", err)
	}
	return nil
}

func (s *Store) ListUptimeResults(ctx context.Context, monitorID string, from, to time.Time) ([]appdb.UptimeResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, monitor_id, checked_at, status, response_time_ms, error
		 FROM uptime_results
		 WHERE monitor_id = ? AND checked_at >= ? AND checked_at <= ?
		 ORDER BY checked_at ASC`,
		monitorID, tsText(from), tsText(to))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list uptime results: %w", err)
	}
	defer rows.Close()
	var out []appdb.UptimeResult
	for rows.Next() {
		r, err := scanUptimeResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) LatestUptimeResult(ctx context.Context, monitorID string) (appdb.UptimeResult, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, monitor_id, checked_at, status, response_time_ms, error
		 FROM uptime_results WHERE monitor_id = ? ORDER BY checked_at DESC LIMIT 1`,
		monitorID)
	r, err := scanUptimeResult(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.UptimeResult{}, appdb.ErrNotFound
	}
	return r, err
}

func (s *Store) PruneUptimeResultsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM uptime_results WHERE checked_at < ?`, tsText(cutoff))
	if err != nil {
		return 0, fmt.Errorf("sqlite: prune uptime results: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func scanUptimeResult(sc rowScanner) (appdb.UptimeResult, error) {
	var r appdb.UptimeResult
	var checkedAt sql.NullString
	if err := sc.Scan(&r.ID, &r.MonitorID, &checkedAt, &r.Status, &r.ResponseTimeMs, &r.Error); err != nil {
		return appdb.UptimeResult{}, fmt.Errorf("sqlite: scan uptime result: %w", err)
	}
	var err error
	if r.CheckedAt, err = scanTS(checkedAt); err != nil {
		return appdb.UptimeResult{}, err
	}
	return r, nil
}


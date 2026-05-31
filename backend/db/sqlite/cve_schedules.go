package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const cveScheduleColumns = `id, target_type, target_id, label, interval_seconds, enabled, created_by, created_at, last_run_at`

func (s *Store) CreateCveSchedule(ctx context.Context, r appdb.CveSchedule) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cve_schedules (`+cveScheduleColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TargetType, r.TargetID, r.Label, r.IntervalSeconds,
		boolToInt(r.Enabled), r.CreatedBy, tsText(r.CreatedAt), nullTSText(r.LastRunAt))
	if err != nil {
		return fmt.Errorf("sqlite: create cve_schedule: %w", err)
	}
	return nil
}

func scanCveSchedule(sc rowScanner) (appdb.CveSchedule, error) {
	var r appdb.CveSchedule
	var createdAt string
	var lastRunAt sql.NullString
	var enabled int
	if err := sc.Scan(&r.ID, &r.TargetType, &r.TargetID, &r.Label, &r.IntervalSeconds,
		&enabled, &r.CreatedBy, &createdAt, &lastRunAt); err != nil {
		return appdb.CveSchedule{}, err
	}
	r.Enabled = enabled != 0
	r.CreatedAt, _ = parseTS(createdAt)
	if t, err := scanNullTS(lastRunAt); err == nil {
		r.LastRunAt = t
	}
	return r, nil
}

func (s *Store) ListCveSchedules(ctx context.Context) ([]appdb.CveSchedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cveScheduleColumns+` FROM cve_schedules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list cve_schedules: %w", err)
	}
	defer rows.Close()
	var out []appdb.CveSchedule
	for rows.Next() {
		r, err := scanCveSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetCveSchedule(ctx context.Context, id string) (appdb.CveSchedule, error) {
	r, err := scanCveSchedule(s.db.QueryRowContext(ctx,
		`SELECT `+cveScheduleColumns+` FROM cve_schedules WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.CveSchedule{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.CveSchedule{}, fmt.Errorf("sqlite: get cve_schedule: %w", err)
	}
	return r, nil
}

func (s *Store) UpdateCveScheduleEnabled(ctx context.Context, id string, enabled bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE cve_schedules SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("sqlite: update cve_schedule enabled: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) UpdateCveScheduleLastRun(ctx context.Context, id string, t time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE cve_schedules SET last_run_at = ? WHERE id = ?`, tsText(t), id)
	if err != nil {
		return fmt.Errorf("sqlite: update cve_schedule last_run_at: %w", err)
	}
	return nil
}

func (s *Store) DeleteCveSchedule(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cve_schedules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete cve_schedule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

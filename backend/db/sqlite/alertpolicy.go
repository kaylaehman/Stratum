package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// --- AlertPolicy CRUD ---

// ListAlertPolicies returns all policy rows ordered by created_at.
func (s *Store) ListAlertPolicies(ctx context.Context) ([]appdb.AlertPolicy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, enabled, min_severity, channels_json, match_json,
		        quiet_hours_json, dedup_window_sec, escalate_json, created_at, updated_at
		 FROM alert_policies ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list alert policies: %w", err)
	}
	defer rows.Close()
	var out []appdb.AlertPolicy
	for rows.Next() {
		p, err := scanAlertPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetAlertPolicy returns a single policy. Returns db.ErrNotFound when absent.
func (s *Store) GetAlertPolicy(ctx context.Context, id string) (appdb.AlertPolicy, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, enabled, min_severity, channels_json, match_json,
		        quiet_hours_json, dedup_window_sec, escalate_json, created_at, updated_at
		 FROM alert_policies WHERE id = ?`, id)
	p, err := scanAlertPolicy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.AlertPolicy{}, appdb.ErrNotFound
	}
	return p, err
}

// CreateAlertPolicy inserts a new policy row.
func (s *Store) CreateAlertPolicy(ctx context.Context, p appdb.AlertPolicy) error {
	chJSON, _ := json.Marshal(p.Channels)
	matchJSON, _ := json.Marshal(p.Match)
	qhJSON := marshalNullable(p.QuietHours)
	escJSON := marshalNullable(p.Escalate)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alert_policies
		   (id, name, enabled, min_severity, channels_json, match_json,
		    quiet_hours_json, dedup_window_sec, escalate_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, boolToInt(p.Enabled), p.MinSeverity,
		string(chJSON), string(matchJSON), qhJSON, p.DedupWindowSec, escJSON,
		tsText(p.CreatedAt), tsText(p.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create alert policy: %w", err)
	}
	return nil
}

// UpdateAlertPolicy replaces all mutable columns for an existing policy.
func (s *Store) UpdateAlertPolicy(ctx context.Context, p appdb.AlertPolicy) error {
	chJSON, _ := json.Marshal(p.Channels)
	matchJSON, _ := json.Marshal(p.Match)
	qhJSON := marshalNullable(p.QuietHours)
	escJSON := marshalNullable(p.Escalate)
	res, err := s.db.ExecContext(ctx,
		`UPDATE alert_policies
		 SET name=?, enabled=?, min_severity=?, channels_json=?, match_json=?,
		     quiet_hours_json=?, dedup_window_sec=?, escalate_json=?, updated_at=?
		 WHERE id=?`,
		p.Name, boolToInt(p.Enabled), p.MinSeverity,
		string(chJSON), string(matchJSON), qhJSON, p.DedupWindowSec, escJSON,
		tsText(p.UpdatedAt), p.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update alert policy: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// DeleteAlertPolicy removes a policy row.
func (s *Store) DeleteAlertPolicy(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM alert_policies WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete alert policy: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// --- AlertDelivery ---

// InsertAlertDelivery appends a delivery record.
func (s *Store) InsertAlertDelivery(ctx context.Context, d appdb.AlertDelivery) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alert_deliveries (id, policy_id, alert_key, severity, channel, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.PolicyID, d.AlertKey, d.Severity, d.Channel, d.Status, tsText(d.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert alert delivery: %w", err)
	}
	return nil
}

// ListAlertDeliveries returns at most limit rows newest-first (0 = default 100).
func (s *Store) ListAlertDeliveries(ctx context.Context, limit int) ([]appdb.AlertDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, policy_id, alert_key, severity, channel, status, created_at
		 FROM alert_deliveries ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list alert deliveries: %w", err)
	}
	defer rows.Close()
	var out []appdb.AlertDelivery
	for rows.Next() {
		d, err := scanAlertDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// LatestDeliveryForKey returns the most-recent delivered row for an alert key.
func (s *Store) LatestDeliveryForKey(ctx context.Context, alertKey string) (appdb.AlertDelivery, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, policy_id, alert_key, severity, channel, status, created_at
		 FROM alert_deliveries
		 WHERE alert_key=? AND status=?
		 ORDER BY created_at DESC LIMIT 1`,
		alertKey, appdb.AlertDeliveryStatusDelivered)
	d, err := scanAlertDelivery(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.AlertDelivery{}, appdb.ErrNotFound
	}
	return d, err
}

// --- scan helpers ---

type alertPolicyScanner interface {
	Scan(dest ...any) error
}

func scanAlertPolicy(r alertPolicyScanner) (appdb.AlertPolicy, error) {
	var (
		p           appdb.AlertPolicy
		enabled     int
		chJSON      string
		matchJSON   string
		qhJSON      sql.NullString
		escJSON     sql.NullString
		createdAt   string
		updatedAt   string
	)
	err := r.Scan(
		&p.ID, &p.Name, &enabled, &p.MinSeverity,
		&chJSON, &matchJSON, &qhJSON, &p.DedupWindowSec, &escJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return appdb.AlertPolicy{}, err
	}
	p.Enabled = enabled != 0

	if e := json.Unmarshal([]byte(chJSON), &p.Channels); e != nil {
		p.Channels = []string{}
	}
	if e := json.Unmarshal([]byte(matchJSON), &p.Match); e != nil {
		p.Match = appdb.AlertPolicyMatch{}
	}
	if qhJSON.Valid && qhJSON.String != "" && qhJSON.String != "null" {
		var qh appdb.AlertQuietHours
		if e := json.Unmarshal([]byte(qhJSON.String), &qh); e == nil {
			p.QuietHours = &qh
		}
	}
	if escJSON.Valid && escJSON.String != "" && escJSON.String != "null" {
		var esc appdb.AlertEscalate
		if e := json.Unmarshal([]byte(escJSON.String), &esc); e == nil {
			p.Escalate = &esc
		}
	}
	if t, e := parseTS(createdAt); e == nil {
		p.CreatedAt = t
	}
	if t, e := parseTS(updatedAt); e == nil {
		p.UpdatedAt = t
	}
	return p, nil
}

type alertDeliveryScanner interface {
	Scan(dest ...any) error
}

func scanAlertDelivery(r alertDeliveryScanner) (appdb.AlertDelivery, error) {
	var (
		d         appdb.AlertDelivery
		createdAt string
	)
	if err := r.Scan(&d.ID, &d.PolicyID, &d.AlertKey, &d.Severity, &d.Channel, &d.Status, &createdAt); err != nil {
		return appdb.AlertDelivery{}, err
	}
	if t, e := parseTS(createdAt); e == nil {
		d.CreatedAt = t
	}
	return d, nil
}

// marshalNullable marshals v to a JSON string or returns nil if v is nil.
func marshalNullable(v any) any {
	if v == nil {
		return nil
	}
	// Use reflect-free approach: marshal and check for "null"
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return nil
	}
	return string(b)
}

var _ = time.Now // ensure time import is used

// Package sqlite is the SQLite-backed implementation of db.Store. It is
// hand-written against database/sql (the spec's sqlc adoption is deferred; the
// db.Store interface preserves the seam so it remains a drop-in later).
//
// Timestamps are persisted explicitly as RFC3339Nano UTC text and parsed with a
// tolerant helper, avoiding modernc/SQLite date-affinity ambiguity.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// Store implements appdb.Store over a *sql.DB.
type Store struct {
	db *sql.DB
}

// New wraps an open *sql.DB.
func New(db *sql.DB) *Store { return &Store{db: db} }

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

const tsLayout = time.RFC3339Nano

func tsText(t time.Time) string { return t.UTC().Format(tsLayout) }

func nullTSText(t *time.Time) any {
	if t == nil {
		return nil
	}
	return tsText(*t)
}

// parseTS tolerantly parses timestamps written by this store (RFC3339Nano) or
// by SQLite's CURRENT_TIMESTAMP default ("2006-01-02 15:04:05").
func parseTS(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("sqlite: unparseable timestamp %q", s)
}

func scanTS(ns sql.NullString) (time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return time.Time{}, nil
	}
	return parseTS(ns.String)
}

func scanNullTS(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := parseTS(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func ptrFromNull(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

// --- Users ---

func (s *Store) CreateUser(ctx context.Context, u appdb.User) error {
	created := u.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, nullableEmpty(u.Email), u.PasswordHash, u.Role, tsText(created))
	if err != nil {
		return fmt.Errorf("sqlite: create user: %w", err)
	}
	return nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (appdb.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, role, created_at FROM users WHERE id = ?`, id))
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (appdb.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, role, created_at FROM users WHERE username = ?`, username))
}

func (s *Store) scanUser(row *sql.Row) (appdb.User, error) {
	var u appdb.User
	var email sql.NullString
	var createdAt sql.NullString
	err := row.Scan(&u.ID, &u.Username, &email, &u.PasswordHash, &u.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.User{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.User{}, fmt.Errorf("sqlite: scan user: %w", err)
	}
	u.Email = email.String
	if u.CreatedAt, err = scanTS(createdAt); err != nil {
		return appdb.User{}, err
	}
	return u, nil
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("sqlite: count users: %w", err)
	}
	return n, nil
}

// --- Sessions ---

func (s *Store) CreateSession(ctx context.Context, sess appdb.Session) error {
	created := sess.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, refresh_hash, user_agent, ip, created_at, expires_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.RefreshHash,
		nullableEmpty(sess.UserAgent), nullableEmpty(sess.IP),
		tsText(created), tsText(sess.ExpiresAt), nullTSText(sess.RevokedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create session: %w", err)
	}
	return nil
}

func (s *Store) GetSession(ctx context.Context, id string) (appdb.Session, error) {
	var sess appdb.Session
	var ua, ip, createdAt, expiresAt, revokedAt sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, refresh_hash, user_agent, ip, created_at, expires_at, revoked_at
		 FROM sessions WHERE id = ?`, id).
		Scan(&sess.ID, &sess.UserID, &sess.RefreshHash, &ua, &ip, &createdAt, &expiresAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.Session{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.Session{}, fmt.Errorf("sqlite: scan session: %w", err)
	}
	sess.UserAgent = ua.String
	sess.IP = ip.String
	if sess.CreatedAt, err = scanTS(createdAt); err != nil {
		return appdb.Session{}, err
	}
	if sess.ExpiresAt, err = scanTS(expiresAt); err != nil {
		return appdb.Session{}, err
	}
	if sess.RevokedAt, err = scanNullTS(revokedAt); err != nil {
		return appdb.Session{}, err
	}
	return sess, nil
}

func (s *Store) RevokeSession(ctx context.Context, id string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE id = ?`, tsText(at), id)
	if err != nil {
		return fmt.Errorf("sqlite: revoke session: %w", err)
	}
	return nil
}

// --- Activity (append-only) ---

func (s *Store) AppendActivity(ctx context.Context, e appdb.ActivityEntry) error {
	created := e.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO activity_log (id, user_id, action, target_type, target_id, detail_json, result, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, nullStr(e.UserID), e.Action, nullStr(e.TargetType), nullStr(e.TargetID),
		nullStr(e.DetailJSON), e.Result, tsText(created))
	if err != nil {
		return fmt.Errorf("sqlite: append activity: %w", err)
	}
	return nil
}

func (s *Store) ListActivity(ctx context.Context, f appdb.ActivityFilter) ([]appdb.ActivityEntry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, user_id, action, target_type, target_id, detail_json, result, created_at
	      FROM activity_log WHERE 1=1`
	var args []any
	if f.UserID != nil {
		q += ` AND user_id = ?`
		args = append(args, *f.UserID)
	}
	if f.Action != nil {
		q += ` AND action = ?`
		args = append(args, *f.Action)
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list activity: %w", err)
	}
	defer rows.Close()

	var out []appdb.ActivityEntry
	for rows.Next() {
		var e appdb.ActivityEntry
		var userID, targetType, targetID, detail, createdAt sql.NullString
		if err := rows.Scan(&e.ID, &userID, &e.Action, &targetType, &targetID, &detail, &e.Result, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan activity: %w", err)
		}
		e.UserID = ptrFromNull(userID)
		e.TargetType = ptrFromNull(targetType)
		e.TargetID = ptrFromNull(targetID)
		e.DetailJSON = ptrFromNull(detail)
		if e.CreatedAt, err = scanTS(createdAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// nullableEmpty stores "" as SQL NULL for optional text columns.
func nullableEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

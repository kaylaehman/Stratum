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
	"strings"
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

func (s *Store) CountUsersByRole(ctx context.Context, role string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ?`, role).Scan(&n); err != nil {
		return 0, fmt.Errorf("sqlite: count users by role: %w", err)
	}
	return n, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]appdb.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, email, password_hash, role, created_at FROM users ORDER BY created_at, username`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list users: %w", err)
	}
	defer rows.Close()
	var out []appdb.User
	for rows.Next() {
		var u appdb.User
		var email, createdAt sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &email, &u.PasswordHash, &u.Role, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan user: %w", err)
		}
		u.Email = email.String
		if u.CreatedAt, err = scanTS(createdAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUserRole(ctx context.Context, id, role string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, id)
	if err != nil {
		return fmt.Errorf("sqlite: update user role: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("sqlite: update password hash: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserProfile(ctx context.Context, id, username, email string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ?, email = ? WHERE id = ?`,
		username, nullableEmpty(email), id)
	if err != nil {
		return fmt.Errorf("sqlite: update user profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete user: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
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

// ListSessionsByUser returns a user's sessions (newest first), including
// revoked/expired ones so the UI can show session history.
func (s *Store) ListSessionsByUser(ctx context.Context, userID string) ([]appdb.Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, refresh_hash, user_agent, ip, created_at, expires_at, revoked_at
		 FROM sessions WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list sessions: %w", err)
	}
	defer rows.Close()
	var out []appdb.Session
	for rows.Next() {
		var sess appdb.Session
		var ua, ip, createdAt, expiresAt, revokedAt sql.NullString
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.RefreshHash, &ua, &ip, &createdAt, &expiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan session: %w", err)
		}
		sess.UserAgent = ua.String
		sess.IP = ip.String
		if sess.CreatedAt, err = scanTS(createdAt); err != nil {
			return nil, err
		}
		if sess.ExpiresAt, err = scanTS(expiresAt); err != nil {
			return nil, err
		}
		if sess.RevokedAt, err = scanNullTS(revokedAt); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// RevokeAllUserSessions revokes every not-yet-revoked session for a user (used
// when an admin deletes the account; stops refresh-token rotation immediately).
func (s *Store) RevokeAllUserSessions(ctx context.Context, userID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, tsText(at), userID)
	if err != nil {
		return fmt.Errorf("sqlite: revoke user sessions: %w", err)
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
	q := `SELECT rowid, id, user_id, action, target_type, target_id, detail_json, result, created_at
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
	q += ` ORDER BY rowid DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list activity: %w", err)
	}
	defer rows.Close()

	var out []appdb.ActivityEntry
	for rows.Next() {
		e, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// QueryActivityLog runs the filtered, keyset-paginated audit query behind
// GET /api/activity. Ordering is by rowid DESC (strict insertion order — the
// canonical "order things happened"); the cursor seeks past a rowid. Every
// filter is a bound parameter (no string interpolation); the q substring is an
// escaped LIKE so a literal `%`/`_` in a path can't widen the match.
func (s *Store) QueryActivityLog(ctx context.Context, f appdb.ActivityQuery) ([]appdb.ActivityEntry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT rowid, id, user_id, action, target_type, target_id, detail_json, result, created_at
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
	if f.ActionPrefix != nil {
		q += ` AND action LIKE ? ESCAPE '\'`
		args = append(args, escapeLike(*f.ActionPrefix)+"%")
	}
	if f.TargetType != nil {
		q += ` AND target_type = ?`
		args = append(args, *f.TargetType)
	}
	if f.TargetID != nil {
		q += ` AND target_id = ?`
		args = append(args, *f.TargetID)
	}
	if f.Result != nil {
		q += ` AND result = ?`
		args = append(args, *f.Result)
	}
	if f.From != nil {
		q += ` AND created_at >= ?`
		args = append(args, tsText(*f.From))
	}
	if f.To != nil {
		q += ` AND created_at <= ?`
		args = append(args, tsText(*f.To))
	}
	if f.Q != "" {
		like := "%" + escapeLike(f.Q) + "%"
		q += ` AND (action LIKE ? ESCAPE '\' OR target_id LIKE ? ESCAPE '\' OR detail_json LIKE ? ESCAPE '\')`
		args = append(args, like, like, like)
	}
	if f.CursorRowID != nil {
		q += ` AND rowid < ?`
		args = append(args, *f.CursorRowID)
	}
	q += ` ORDER BY rowid DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query activity: %w", err)
	}
	defer rows.Close()

	var out []appdb.ActivityEntry
	for rows.Next() {
		e, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// escapeLike escapes the LIKE wildcards (% and _) and the escape char itself so
// the value matches literally under `ESCAPE '\'`.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func scanActivity(sc rowScanner) (appdb.ActivityEntry, error) {
	var e appdb.ActivityEntry
	var userID, targetType, targetID, detail, createdAt sql.NullString
	if err := sc.Scan(&e.RowID, &e.ID, &userID, &e.Action, &targetType, &targetID, &detail, &e.Result, &createdAt); err != nil {
		return appdb.ActivityEntry{}, fmt.Errorf("sqlite: scan activity: %w", err)
	}
	e.UserID = ptrFromNull(userID)
	e.TargetType = ptrFromNull(targetType)
	e.TargetID = ptrFromNull(targetID)
	e.DetailJSON = ptrFromNull(detail)
	var err error
	if e.CreatedAt, err = scanTS(createdAt); err != nil {
		return appdb.ActivityEntry{}, err
	}
	return e, nil
}

// nullableEmpty stores "" as SQL NULL for optional text columns.
func nullableEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

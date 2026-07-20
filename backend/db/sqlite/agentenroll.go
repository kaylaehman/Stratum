package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// enrollAction is the only action an enrollment token is valid for.
const enrollAction = "agent-install"

// timeFmt is the on-disk timestamp format (UTC RFC3339 sorts lexically =
// chronologically, so string comparison in SQL is correct for expiry checks).
const timeFmt = time.RFC3339

// CreateEnrollToken persists a new single-use enrollment token (hash only).
func (s *Store) CreateEnrollToken(ctx context.Context, t appdb.EnrollToken) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_enroll_tokens
		   (id, node_id, token_hash, action, created_by, created_at, expires_at, used_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		t.ID, t.NodeID, t.TokenHash, t.Action, t.CreatedBy,
		t.CreatedAt.UTC().Format(timeFmt), t.ExpiresAt.UTC().Format(timeFmt))
	if err != nil {
		return fmt.Errorf("sqlite: create enroll token: %w", err)
	}
	return nil
}

// ConsumeEnrollToken atomically marks a token used and reports whether it was
// consumed. It is the authorization gate for enrollment: a return of false means
// the token is unknown, already used, expired, or for a different node/action —
// the caller MUST treat false as a hard auth failure. The single UPDATE closes
// the check-then-act race that a SELECT-then-UPDATE would open.
func (s *Store) ConsumeEnrollToken(ctx context.Context, nodeID, tokenHash string) (bool, error) {
	now := time.Now().UTC().Format(timeFmt)
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_enroll_tokens
		    SET used_at = ?
		  WHERE token_hash = ? AND node_id = ? AND action = ?
		    AND used_at IS NULL AND expires_at > ?`,
		now, tokenHash, nodeID, enrollAction, now)
	if err != nil {
		return false, fmt.Errorf("sqlite: consume enroll token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("sqlite: consume enroll token rows: %w", err)
	}
	return n == 1, nil
}

// ValidateEnrollToken reports whether a token is currently valid for the node
// WITHOUT consuming it — used to gate the (non-secret) binary download so the
// same token can later be consumed exactly once by enrollment.
func (s *Store) ValidateEnrollToken(ctx context.Context, nodeID, tokenHash string) (bool, error) {
	now := time.Now().UTC().Format(timeFmt)
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM agent_enroll_tokens
		  WHERE token_hash = ? AND node_id = ? AND action = ?
		    AND used_at IS NULL AND expires_at > ?
		  LIMIT 1`,
		tokenHash, nodeID, enrollAction, now).Scan(&one)
	if err != nil {
		// sql.ErrNoRows → not valid (not an error condition for the caller).
		return false, nil //nolint:nilerr // absence of a row means "not valid", not a failure
	}
	return true, nil
}

// PurgeExpiredEnrollTokens deletes used or expired tokens, returning the count.
func (s *Store) PurgeExpiredEnrollTokens(ctx context.Context) (int64, error) {
	now := time.Now().UTC().Format(timeFmt)
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_enroll_tokens WHERE used_at IS NOT NULL OR expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("sqlite: purge enroll tokens: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func (s *Store) CreateBookmark(ctx context.Context, b appdb.Bookmark) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bookmarks (id, user_id, label, resource_type, resource_ref, group_name, order_index, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.UserID, b.Label, b.ResourceType, b.ResourceRef, b.GroupName, b.OrderIndex, tsText(b.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create bookmark: %w", err)
	}
	return nil
}

func (s *Store) ListBookmarksByUser(ctx context.Context, userID string) ([]appdb.Bookmark, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, label, resource_type, resource_ref, group_name, order_index, created_at
		 FROM bookmarks WHERE user_id = ? ORDER BY order_index ASC, created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list bookmarks: %w", err)
	}
	defer rows.Close()
	var out []appdb.Bookmark
	for rows.Next() {
		var b appdb.Bookmark
		var createdAt string
		if err := rows.Scan(&b.ID, &b.UserID, &b.Label, &b.ResourceType, &b.ResourceRef, &b.GroupName, &b.OrderIndex, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan bookmark: %w", err)
		}
		b.CreatedAt, _ = parseTS(createdAt)
		out = append(out, b)
	}
	return out, rows.Err()
}

// DeleteBookmark removes a bookmark only if it belongs to userID (IDOR-safe).
func (s *Store) DeleteBookmark(ctx context.Context, id, userID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("sqlite: delete bookmark: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// SetBookmarkOrder updates order_index for the given ids in a transaction,
// scoped to userID so a caller can only reorder their own bookmarks.
func (s *Store) SetBookmarkOrder(ctx context.Context, userID string, orderedIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE bookmarks SET order_index = ? WHERE id = ? AND user_id = ?`, i, id, userID); err != nil {
			return fmt.Errorf("sqlite: reorder bookmark: %w", err)
		}
	}
	return tx.Commit()
}

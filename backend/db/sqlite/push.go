package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// GetVAPID returns the stored VAPID keypair (id=1).
// Returns appdb.ErrNotFound if no row exists yet.
func (s *Store) GetVAPID(ctx context.Context) (appdb.PushVAPID, error) {
	var v appdb.PushVAPID
	err := s.db.QueryRowContext(ctx,
		`SELECT private_key, public_key FROM push_vapid WHERE id = 1`).
		Scan(&v.PrivateKey, &v.PublicKey)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.PushVAPID{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.PushVAPID{}, fmt.Errorf("sqlite: get vapid: %w", err)
	}
	return v, nil
}

// UpsertVAPID inserts or replaces the VAPID keypair (singleton row id=1).
func (s *Store) UpsertVAPID(ctx context.Context, v appdb.PushVAPID) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO push_vapid (id, private_key, public_key) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET private_key=excluded.private_key, public_key=excluded.public_key`,
		v.PrivateKey, v.PublicKey)
	if err != nil {
		return fmt.Errorf("sqlite: upsert vapid: %w", err)
	}
	return nil
}

// ListPushSubscriptions returns all push subscriptions.
func (s *Store) ListPushSubscriptions(ctx context.Context) ([]appdb.PushSubscription, error) {
	return s.queryPushSubs(ctx,
		`SELECT id, user_id, endpoint, p256dh, auth, created_at FROM push_subscriptions ORDER BY created_at ASC`,
	)
}

// ListPushSubscriptionsByUser returns all push subscriptions for a given user.
func (s *Store) ListPushSubscriptionsByUser(ctx context.Context, userID string) ([]appdb.PushSubscription, error) {
	return s.queryPushSubs(ctx,
		`SELECT id, user_id, endpoint, p256dh, auth, created_at FROM push_subscriptions WHERE user_id = ? ORDER BY created_at ASC`,
		userID,
	)
}

func (s *Store) queryPushSubs(ctx context.Context, q string, args ...any) ([]appdb.PushSubscription, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list push subscriptions: %w", err)
	}
	defer rows.Close()
	var out []appdb.PushSubscription
	for rows.Next() {
		var sub appdb.PushSubscription
		var createdAt string
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan push subscription: %w", err)
		}
		sub.CreatedAt, _ = parseTS(createdAt)
		out = append(out, sub)
	}
	return out, rows.Err()
}

// UpsertPushSubscription inserts or replaces a push subscription by endpoint.
func (s *Store) UpsertPushSubscription(ctx context.Context, sub appdb.PushSubscription) error {
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(endpoint) DO UPDATE SET
		   user_id=excluded.user_id,
		   p256dh=excluded.p256dh,
		   auth=excluded.auth`,
		sub.ID, sub.UserID, sub.Endpoint, sub.P256DH, sub.Auth, tsText(sub.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: upsert push subscription: %w", err)
	}
	return nil
}

// DeletePushSubscription removes a push subscription by endpoint URL.
func (s *Store) DeletePushSubscription(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	if err != nil {
		return fmt.Errorf("sqlite: delete push subscription: %w", err)
	}
	return nil
}

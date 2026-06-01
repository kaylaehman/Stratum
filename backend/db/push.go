package db

import (
	"context"
	"time"
)

// PushSubscription stores a single browser push endpoint for a user.
type PushSubscription struct {
	ID        string
	UserID    string
	Endpoint  string
	P256DH    string
	Auth      string
	CreatedAt time.Time
}

// PushVAPID stores the server's VAPID keypair (only one row; id=1).
type PushVAPID struct {
	PrivateKey string
	PublicKey  string
}

// PushStore is the narrow DB interface the push package needs.
// Implemented by *sqlite.Store; not part of the central db.Store interface.
type PushStore interface {
	GetVAPID(ctx context.Context) (PushVAPID, error)
	UpsertVAPID(ctx context.Context, v PushVAPID) error
	ListPushSubscriptions(ctx context.Context) ([]PushSubscription, error)
	ListPushSubscriptionsByUser(ctx context.Context, userID string) ([]PushSubscription, error)
	UpsertPushSubscription(ctx context.Context, sub PushSubscription) error
	DeletePushSubscription(ctx context.Context, endpoint string) error
}

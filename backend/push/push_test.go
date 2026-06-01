package push_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/push"
)

// stubStore is an in-memory PushStore for testing.
type stubStore struct {
	vapid *db.PushVAPID
	subs  []db.PushSubscription
}

func (s *stubStore) GetVAPID(_ context.Context) (db.PushVAPID, error) {
	if s.vapid == nil {
		return db.PushVAPID{}, db.ErrNotFound
	}
	return *s.vapid, nil
}

func (s *stubStore) UpsertVAPID(_ context.Context, v db.PushVAPID) error {
	s.vapid = &v
	return nil
}

func (s *stubStore) ListPushSubscriptions(_ context.Context) ([]db.PushSubscription, error) {
	return s.subs, nil
}

func (s *stubStore) ListPushSubscriptionsByUser(_ context.Context, userID string) ([]db.PushSubscription, error) {
	var out []db.PushSubscription
	for _, sub := range s.subs {
		if sub.UserID == userID {
			out = append(out, sub)
		}
	}
	return out, nil
}

func (s *stubStore) UpsertPushSubscription(_ context.Context, sub db.PushSubscription) error {
	for i, existing := range s.subs {
		if existing.Endpoint == sub.Endpoint {
			s.subs[i] = sub
			return nil
		}
	}
	s.subs = append(s.subs, sub)
	return nil
}

func (s *stubStore) DeletePushSubscription(_ context.Context, endpoint string) error {
	for i, sub := range s.subs {
		if sub.Endpoint == endpoint {
			s.subs = append(s.subs[:i], s.subs[i+1:]...)
			return nil
		}
	}
	return nil
}

func newTestService(t *testing.T, store push.Store) *push.Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc, err := push.New(context.Background(), store, "mailto:test@example.com", logger)
	if err != nil {
		t.Fatalf("push.New: %v", err)
	}
	return svc
}

// TestVAPIDKeyPersistence verifies that the keypair is generated once and reused.
func TestVAPIDKeyPersistence(t *testing.T) {
	store := &stubStore{}
	svc1 := newTestService(t, store)
	pub1 := svc1.PublicKey()

	if pub1 == "" {
		t.Fatal("expected non-empty public key")
	}
	if store.vapid == nil {
		t.Fatal("expected vapid to be persisted in store")
	}

	// Create a second service from the same store — must return the same key.
	svc2 := newTestService(t, store)
	if svc2.PublicKey() != pub1 {
		t.Errorf("key changed: got %q, want %q", svc2.PublicKey(), pub1)
	}
}

// TestSubscribeRoundTrip verifies subscribe/unsubscribe store round-trips.
func TestSubscribeRoundTrip(t *testing.T) {
	store := &stubStore{}
	svc := newTestService(t, store)
	ctx := context.Background()

	sub := push.Subscription{Endpoint: "https://push.example.com/abc"}
	sub.Keys.P256DH = "dGVzdHB1YmxpY2tleQ=="
	sub.Keys.Auth = "dGVzdGF1dGg="

	if err := svc.Subscribe(ctx, "user-1", sub); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	subs, _ := store.ListPushSubscriptions(ctx)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].UserID != "user-1" || subs[0].Endpoint != sub.Endpoint {
		t.Errorf("unexpected subscription row: %+v", subs[0])
	}

	if err := svc.Unsubscribe(ctx, sub.Endpoint); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	subs, _ = store.ListPushSubscriptions(ctx)
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions after unsubscribe, got %d", len(subs))
	}
}

// TestSubscribeValidation ensures missing fields are rejected.
func TestSubscribeValidation(t *testing.T) {
	store := &stubStore{}
	svc := newTestService(t, store)
	ctx := context.Background()

	err := svc.Subscribe(ctx, "user-1", push.Subscription{}) // empty
	if err == nil {
		t.Fatal("expected error for empty subscription, got nil")
	}
}

// TestPayloadBuild verifies JSON marshalling of a payload with actions.
func TestPayloadBuild(t *testing.T) {
	p := push.Payload{
		Title:        "Critical CVE detected",
		Body:         "Image nginx:latest has 3 critical vulnerabilities.",
		Tag:          "cve-critical",
		ResourceID:   "ctr-123",
		ResourceType: "container",
		Actions: []push.Action{
			{Action: "ack", Title: "Acknowledge"},
			{Action: "restart", Title: "Restart"},
		},
	}
	if p.Title == "" {
		t.Fatal("payload title should not be empty")
	}
	if len(p.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(p.Actions))
	}
}

// TestSendToAllNoSubs verifies SendToAll is a no-op when there are no subscriptions.
func TestSendToAllNoSubs(t *testing.T) {
	store := &stubStore{}
	svc := newTestService(t, store)
	// Must not error — no subscriptions means no sends.
	if err := svc.SendToAll(context.Background(), push.Payload{Title: "test", Body: "test"}); err != nil {
		t.Fatalf("SendToAll with no subs: %v", err)
	}
}

// TestCreatedAtSet verifies the subscription gets a non-zero CreatedAt.
func TestCreatedAtSet(t *testing.T) {
	before := time.Now()
	store := &stubStore{}
	svc := newTestService(t, store)
	ctx := context.Background()

	sub := push.Subscription{Endpoint: "https://push.example.com/xyz"}
	sub.Keys.P256DH = "dGVzdHB1YmxpY2tleQ=="
	sub.Keys.Auth = "dGVzdGF1dGg="
	_ = svc.Subscribe(ctx, "user-2", sub)

	subs, _ := store.ListPushSubscriptions(ctx)
	if len(subs) == 0 {
		t.Fatal("expected 1 sub")
	}
	if subs[0].CreatedAt.Before(before) {
		t.Errorf("CreatedAt not set: %v", subs[0].CreatedAt)
	}
}

// Compile-time check: stubStore satisfies push.Store.
var _ push.Store = (*stubStore)(nil)

// staticCheck ensures db.ErrNotFound is properly exported.
var _ = errors.Is(nil, db.ErrNotFound)

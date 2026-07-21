package secrets

import (
	"context"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// stubExpiryStore is an in-memory SecretExpiryStore for tests.
type stubExpiryStore struct {
	rows map[string]*db.SecretExpiryRow // keyed by SecretID
}

func newStubExpiryStore() *stubExpiryStore {
	return &stubExpiryStore{rows: make(map[string]*db.SecretExpiryRow)}
}

func (s *stubExpiryStore) seed(id, key, groupID string, expiresAt *time.Time) {
	t := time.Now().Add(-24 * time.Hour) // created 1 day ago
	s.rows[id] = &db.SecretExpiryRow{
		SecretID:  id,
		Key:       key,
		GroupID:   groupID,
		ExpiresAt: expiresAt,
		CreatedAt: t,
	}
}

func (s *stubExpiryStore) SetSecretExpiry(_ context.Context, id string, expiresAt *time.Time) error {
	r, ok := s.rows[id]
	if !ok {
		return db.ErrNotFound
	}
	r.ExpiresAt = expiresAt
	return nil
}

func (s *stubExpiryStore) MarkSecretRotated(_ context.Context, id string, newExpiresAt *time.Time) error {
	r, ok := s.rows[id]
	if !ok {
		return db.ErrNotFound
	}
	now := time.Now()
	r.RotatedAt = &now
	if newExpiresAt != nil {
		r.ExpiresAt = newExpiresAt
	}
	return nil
}

func (s *stubExpiryStore) ListExpiringSecrets(_ context.Context, withinDays int) ([]db.SecretExpiryRow, error) {
	cutoff := time.Now().AddDate(0, 0, withinDays)
	var out []db.SecretExpiryRow
	for _, r := range s.rows {
		if r.ExpiresAt != nil && !r.ExpiresAt.After(cutoff) {
			out = append(out, *r)
		}
	}
	return out, nil
}

// --- ComputeStatus table tests ---

func TestComputeStatus(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      db.ExpiryStatus
	}{
		{
			name:      "no expiry configured",
			expiresAt: nil,
			want:      db.ExpiryStatusNone,
		},
		{
			name:      "already expired",
			expiresAt: ptr(now.Add(-1 * time.Hour)),
			want:      db.ExpiryStatusExpired,
		},
		{
			name:      "expired yesterday",
			expiresAt: ptr(now.AddDate(0, 0, -1)),
			want:      db.ExpiryStatusExpired,
		},
		{
			name:      "within warn window (5 days out)",
			expiresAt: ptr(now.AddDate(0, 0, 5)),
			want:      db.ExpiryStatusWarning,
		},
		{
			name:      "exactly at warn boundary (30 days out)",
			expiresAt: ptr(now.AddDate(0, 0, 30)),
			want:      db.ExpiryStatusWarning,
		},
		{
			name:      "outside warn window (31 days out)",
			expiresAt: ptr(now.AddDate(0, 0, 31)),
			want:      db.ExpiryStatusOK,
		},
		{
			name:      "far future",
			expiresAt: ptr(now.AddDate(1, 0, 0)),
			want:      db.ExpiryStatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := db.SecretExpiryRow{ExpiresAt: tc.expiresAt}
			got := ComputeStatus(row, now)
			if got != tc.want {
				t.Errorf("ComputeStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func ptr(t time.Time) *time.Time { return &t }

// --- ExpiryService check + notify tests ---

func TestExpiryServiceCheck_NoNotifyOnEmpty(t *testing.T) {
	store := newStubExpiryStore()
	svc := NewExpiry(store)
	notified := false
	svc.SetNotify(func(_ context.Context, _, _, _ string) { notified = true })
	if err := svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if notified {
		t.Error("expected no notification when no expiring secrets")
	}
}

func TestExpiryServiceCheck_NotifiesOnExpiring(t *testing.T) {
	store := newStubExpiryStore()
	soon := time.Now().AddDate(0, 0, 7) // 7 days = within warn window
	store.seed("s1", "DB_PASSWORD", "g1", &soon)

	svc := NewExpiry(store)
	var gotTrigger, gotText string
	svc.SetNotify(func(_ context.Context, trigger, _, text string) {
		gotTrigger = trigger
		gotText = text
	})
	if err := svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotTrigger != "secret.expiry" {
		t.Errorf("trigger = %q, want secret.expiry", gotTrigger)
	}
	if gotText == "" {
		t.Error("expected non-empty notification text")
	}
}

func TestExpiryServiceCheck_NotifiesOnExpired(t *testing.T) {
	store := newStubExpiryStore()
	past := time.Now().Add(-24 * time.Hour)
	store.seed("s2", "API_KEY", "g1", &past)

	svc := NewExpiry(store)
	var notified bool
	svc.SetNotify(func(_ context.Context, _, _, _ string) { notified = true })
	if err := svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !notified {
		t.Error("expected notification for expired secret")
	}
}

func TestExpiryService_SetExpiry_NotFound(t *testing.T) {
	store := newStubExpiryStore()
	svc := NewExpiry(store)
	exp := time.Now().Add(30 * 24 * time.Hour)
	err := svc.SetExpiry(context.Background(), "nonexistent", &exp)
	if err != db.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestExpiryService_MarkRotated(t *testing.T) {
	store := newStubExpiryStore()
	store.seed("s3", "TOKEN", "g1", nil)

	svc := NewExpiry(store)
	newExp := time.Now().AddDate(0, 3, 0)
	if err := svc.MarkRotated(context.Background(), "s3", &newExp); err != nil {
		t.Fatal(err)
	}
	row := store.rows["s3"]
	if row.RotatedAt == nil {
		t.Error("expected rotated_at to be set")
	}
	if row.ExpiresAt == nil || !row.ExpiresAt.Equal(newExp) {
		t.Errorf("expected expires_at = %v, got %v", newExp, row.ExpiresAt)
	}
}

package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

// warnDays mirrors the certs package convention: notify when a secret is
// within this many days of its expires_at.
const warnDays = 30

// ExpiryService wraps a SecretExpiryStore with expiry-check and notification
// logic, mirroring the shape of certs.Service.maybeAlert.
type ExpiryService struct {
	store  db.SecretExpiryStore
	notify func(ctx context.Context, trigger, title, text string)
}

// NewExpiry creates an ExpiryService.
func NewExpiry(store db.SecretExpiryStore) *ExpiryService {
	return &ExpiryService{store: store}
}

// SetNotify wires the notification callback fired when secrets are nearing or
// past expiry. The signature matches the certs/volumes/updates notify pattern.
func (e *ExpiryService) SetNotify(fn func(ctx context.Context, trigger, title, text string)) {
	e.notify = fn
}

// SetExpiry updates the expires_at for a secret. Pass nil to clear expiry.
func (e *ExpiryService) SetExpiry(ctx context.Context, id string, expiresAt *time.Time) error {
	return e.store.SetSecretExpiry(ctx, id, expiresAt)
}

// MarkRotated records that a secret was rotated now and optionally sets a new
// expiry deadline.
func (e *ExpiryService) MarkRotated(ctx context.Context, id string, newExpiresAt *time.Time) error {
	return e.store.MarkSecretRotated(ctx, id, newExpiresAt)
}

// ListExpiring returns secrets within the warn window or already expired.
func (e *ExpiryService) ListExpiring(ctx context.Context) ([]db.SecretExpiryRow, error) {
	return e.store.ListExpiringSecrets(ctx, warnDays)
}

// Check queries expiring/expired secrets and fires the notify callback when any
// are found. It is designed to be called on a schedule (e.g. daily). It is a
// no-op when notify is nil.
func (e *ExpiryService) Check(ctx context.Context) error {
	rows, err := e.store.ListExpiringSecrets(ctx, warnDays)
	if err != nil {
		return err
	}
	if len(rows) == 0 || e.notify == nil {
		return nil
	}
	e.maybeAlert(ctx, rows)
	return nil
}

// ComputeStatus returns the ExpiryStatus for a row relative to now.
func ComputeStatus(row db.SecretExpiryRow, now time.Time) db.ExpiryStatus {
	if row.ExpiresAt == nil {
		return db.ExpiryStatusNone
	}
	if now.After(*row.ExpiresAt) {
		return db.ExpiryStatusExpired
	}
	if row.ExpiresAt.Sub(now) <= time.Duration(warnDays)*24*time.Hour {
		return db.ExpiryStatusWarning
	}
	return db.ExpiryStatusOK
}

// maybeAlert fires the notification when secrets are expiring or expired.
// Mirror of certs.Service.maybeAlert.
func (e *ExpiryService) maybeAlert(ctx context.Context, rows []db.SecretExpiryRow) {
	now := time.Now()
	expired, warning := 0, 0
	soonest := -1
	var soonestKey string

	for _, r := range rows {
		if r.ExpiresAt == nil {
			continue
		}
		days := int(r.ExpiresAt.Sub(now).Hours() / 24)
		if now.After(*r.ExpiresAt) {
			expired++
			days = 0
		} else {
			warning++
		}
		if soonest < 0 || days < soonest {
			soonest, soonestKey = days, r.Key
		}
	}

	total := expired + warning
	if total == 0 {
		return
	}

	title := "Secrets expiring soon"
	text := fmt.Sprintf(
		"%d secret(s) within %d days (expired: %d); earliest: %q in %d day(s)",
		total, warnDays, expired, soonestKey, soonest,
	)
	e.notify(ctx, "secret.expiry", title, text)
}

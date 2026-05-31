package db

import (
	"context"
	"time"
)

// SecretExpiryRow is the expiry/rotation metadata for one secret. It is always
// returned alongside the owning SecretRow's id and key — never with the
// encrypted value blob, so it is safe for listing endpoints.
type SecretExpiryRow struct {
	// SecretID is the secrets.id FK.
	SecretID string `json:"secret_id"`
	// Key is the secret's human-readable name (copied from SecretRow).
	Key string `json:"key"`
	// GroupID mirrors secrets.group_id.
	GroupID string `json:"group_id"`
	// RotatedAt is the last time the secret value was rotated. Nil = never.
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
	// ExpiresAt is the operator-set hard expiry deadline. Nil = no expiry.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// CreatedAt is the original secret creation timestamp.
	CreatedAt time.Time `json:"created_at"`
}

// ExpiryStatus is the computed state of a secret relative to now and a
// warn-window duration.
type ExpiryStatus string

const (
	// ExpiryStatusNone means no expires_at is configured.
	ExpiryStatusNone ExpiryStatus = "none"
	// ExpiryStatusOK means expires_at is configured but outside the warn window.
	ExpiryStatusOK ExpiryStatus = "ok"
	// ExpiryStatusWarning means expires_at is within the warn window (but not yet past).
	ExpiryStatusWarning ExpiryStatus = "warning"
	// ExpiryStatusExpired means expires_at is in the past.
	ExpiryStatusExpired ExpiryStatus = "expired"
)

// SecretExpiryStore is the narrow data interface the secrets expiry service
// depends on. It is satisfied by *sqlite.Store and a stub for tests. The
// central db.Store is intentionally NOT extended — expiry queries live behind
// this focused seam to avoid polluting the main interface.
type SecretExpiryStore interface {
	// SetSecretExpiry sets (or clears) the expires_at column for a secret by ID.
	// Passing nil clears the expiry.
	SetSecretExpiry(ctx context.Context, id string, expiresAt *time.Time) error

	// MarkSecretRotated stamps rotated_at = now and optionally extends
	// expires_at for the next rotation cycle. newExpiresAt nil leaves it unchanged.
	MarkSecretRotated(ctx context.Context, id string, newExpiresAt *time.Time) error

	// ListExpiringSecrets returns all secrets whose expires_at is non-NULL and
	// either within the next withinDays calendar days (warning window) or already
	// past (expired). Never returns value_encrypted material.
	ListExpiringSecrets(ctx context.Context, withinDays int) ([]SecretExpiryRow, error)
}

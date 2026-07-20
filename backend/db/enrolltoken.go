package db

import "time"

// EnrollToken is a single-use, short-lived credential authorizing exactly one
// agent enrollment (CSR signing) for one node. Only TokenHash (the SHA-256 of
// the opaque token) is persisted — the token itself is never stored.
type EnrollToken struct {
	ID        string
	NodeID    string
	TokenHash string
	Action    string // "agent-install"
	CreatedBy string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

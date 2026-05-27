package db

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Store reads when no row matches.
var ErrNotFound = errors.New("db: not found")

// User is an application user. MVP is single-user; role is present but not
// enforced until feature 30 (RBAC).
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

// Session is a refresh-token record enabling revocation. RefreshHash is the
// sha256 of the opaque refresh token; the raw token is never stored.
type Session struct {
	ID          string
	UserID      string
	RefreshHash string
	UserAgent   string
	IP          string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

// ActivityEntry is one append-only audit row.
type ActivityEntry struct {
	ID         string
	UserID     *string
	Action     string
	TargetType *string
	TargetID   *string
	DetailJSON *string
	Result     string
	CreatedAt  time.Time
}

// ActivityFilter narrows ListActivity. Zero values mean "no constraint".
type ActivityFilter struct {
	UserID *string
	Action *string
	Limit  int // 0 => default applied by the store
}

// Node is a registered host. CredentialsEncrypted is an opaque sealed blob;
// the raw credentials are never stored or returned. LastError holds only a
// sanitized category (never a raw transport error).
type Node struct {
	ID                   string
	Name                 string
	Type                 string // proxmox | standalone | ssh
	Host                 string
	Port                 int
	AuthMethod           string
	OSType               string
	CapabilitiesJSON     string
	CredentialsEncrypted []byte
	CredentialsVersion   int
	SSHHostKey           string
	ProxmoxEndpoint      string
	ProxmoxTLSInsecure   bool
	DockerEndpoint       string
	Status               string
	LastError            string
	LastSeen             *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Store is the repository seam. Handlers depend on this interface, never on
// *sql.DB, so a future Postgres implementation is additive. All methods return
// Go standard types (never driver-specific nullable wrappers).
type Store interface {
	// Users
	CreateUser(ctx context.Context, u User) error
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	CountUsers(ctx context.Context) (int, error)

	// Sessions
	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	RevokeSession(ctx context.Context, id string, at time.Time) error

	// Activity (append-only; no update/delete by design)
	AppendActivity(ctx context.Context, e ActivityEntry) error
	ListActivity(ctx context.Context, f ActivityFilter) ([]ActivityEntry, error)

	// Nodes
	CreateNode(ctx context.Context, n Node) error
	GetNode(ctx context.Context, id string) (Node, error)
	ListNodes(ctx context.Context) ([]Node, error)
	UpdateNode(ctx context.Context, n Node) error
	DeleteNode(ctx context.Context, id string) error

	Close() error
}

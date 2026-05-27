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

// ActivityEntry is one append-only audit row. RowID is SQLite's implicit
// monotonic rowid, populated on read and used as the keyset-pagination cursor
// (it is strictly insertion-ordered, unlike the random UUID id or the
// variable-precision created_at text).
type ActivityEntry struct {
	ID         string
	RowID      int64
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

// ActivityQuery is the filtered, keyset-paginated query behind GET /api/activity.
// All fields are optional. Ordering is by rowid DESC (newest first); CursorRowID,
// when set, seeks strictly past it. Limit is honored as given (the query layer
// requests limit+1 to detect a next page).
type ActivityQuery struct {
	UserID       *string
	Action       *string // exact match
	ActionPrefix *string // prefix match, e.g. "fs." matches fs.write, fs.delete, ...
	TargetType   *string
	TargetID     *string
	Result       *string
	From         *time.Time // created_at >= From
	To           *time.Time // created_at <= To
	Q            string     // substring over action/target_id/detail_json (escaped LIKE)
	Limit        int        // 0 => default applied by the store
	CursorRowID  *int64     // seek: rowid < CursorRowID
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

// VM is a Proxmox guest (QEMU VM or LXC container).
type VM struct {
	ID          string
	NodeID      string
	Kind        string // qemu | lxc
	ProxmoxVMID int
	ProxmoxNode string
	Name        string
	Status      string
	OSType      string
	Stale       bool
	GoneSince   *time.Time
	LastSeen    time.Time
}

// Container is a Docker container.
type Container struct {
	ID             string
	NodeID         string
	DockerID       string
	Name           string
	Image          string
	ImageID        string
	Status         string
	ComposeProject string
	Stale          bool
	GoneSince      *time.Time
	LastSeen       time.Time
}

// MountRow is one container mount in the bind-mount index.
type MountRow struct {
	ID               string
	NodeID           string
	ContainerID      string
	Type             string // bind | volume | tmpfs | ...
	Source           string
	NormalizedSource string
	VolumeName       string
	Destination      string
	RW               bool
}

// ContainerSecurityRow is a container's classified security posture (SP8).
type ContainerSecurityRow struct {
	ContainerID        string
	NodeID             string
	Privileged         bool
	CapAddAll          bool
	DangerousCaps      []string
	SeccompUnconfined  bool
	ApparmorUnconfined bool
	Devices            []string
	UsernsHost         bool
	PidHost            bool
	NetHost            bool
	RunsAsRoot         bool
	RunUID             int
	ScannedAt          time.Time
}

// PortExposureRow is one published port. IsNew is durable until acknowledged.
// JSON tags: this row is serialized directly in the ports-audit API responses,
// so it must match the snake_case convention used by the rest of the API.
type PortExposureRow struct {
	ID             string     `json:"id"`
	NodeID         string     `json:"node_id"`
	ContainerID    string     `json:"container_id"`
	HostIP         string     `json:"host_ip"`
	HostPort       int        `json:"host_port"`
	ContainerPort  int        `json:"container_port"`
	Protocol       string     `json:"protocol"`
	InterfaceClass string     `json:"interface_class"`
	IsNew          bool       `json:"is_new"`
	NotifiedAt     *time.Time `json:"notified_at,omitempty"`
	FirstSeen      time.Time  `json:"first_seen"`
	LastSeen       time.Time  `json:"last_seen"`
}

// ImageScanRow is a cached CVE scan summary for an image digest (Feature 20).
type ImageScanRow struct {
	ImageDigest string
	Image       string
	ScannedAt   time.Time
	Critical    int
	High        int
	Medium      int
	Low         int
	Unknown     int
}

// CVEResultRow is one vulnerability for a scanned image digest.
type CVEResultRow struct {
	ID               string
	ImageDigest      string
	CVEID            string
	Severity         string
	Package          string
	InstalledVersion string
	FixedVersion     string
	Title            string
}

// ImageUpdateRow is a container's cached image update-availability (Feature 15).
type ImageUpdateRow struct {
	ContainerID   string
	NodeID        string
	Image         string
	Status        string // up_to_date | update_available | unknown
	CurrentDigest string
	LatestDigest  string
	CheckedAt     time.Time
}

// UserTOTP is a user's TOTP 2FA enrollment (Feature 7). SecretEncrypted is
// AES-sealed; RecoveryHashes are bcrypt hashes consumed on use.
type UserTOTP struct {
	UserID          string
	SecretEncrypted []byte
	Enabled         bool
	RecoveryHashes  []string
}

// BackupRow is a backup job + its outcome (Feature 28).
type BackupRow struct {
	ID         string
	NodeID     string
	Kind       string // volume | proxmox
	Target     string
	DestPath   string
	SizeBytes  int64
	Status     string // running | ok | error
	Error      string
	StartedAt  time.Time
	FinishedAt *time.Time
}

// Script is a saved shell script for the script runner (Feature 27).
type Script struct {
	ID          string
	Name        string
	Description string
	Content     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SecretGroup is a named group of secrets (Feature 12).
type SecretGroup struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
}

// SecretRow is one stored secret. ValueEncrypted is an AES-256-GCM sealed blob;
// the plaintext is never stored or listed without an explicit, audited reveal.
type SecretRow struct {
	ID             string
	GroupID        string
	Key            string
	ValueEncrypted []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TemplateVar is one substitution variable in a template (Feature 14).
type TemplateVar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default"`
}

// Template is a saved, versioned Docker Compose stack (Feature 14).
type Template struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	ComposeYAML string
	Variables   []TemplateVar
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TemplateVersion is an immutable snapshot of a template at one version.
type TemplateVersion struct {
	Version     int           `json:"version"`
	ComposeYAML string        `json:"compose_yaml"`
	Variables   []TemplateVar `json:"variables"`
	CreatedAt   time.Time     `json:"created_at"`
}

// WebhookConfig is a Slack/Discord notification target (Feature 26).
type WebhookConfig struct {
	ID        string
	Name      string
	URL       string
	Provider  string // slack | discord
	Triggers  []string
	Enabled   bool
	CreatedAt time.Time
}

// WOLConfig is a node's optional Wake-on-LAN settings (Feature 6).
type WOLConfig struct {
	NodeID    string
	MAC       string
	Broadcast string
	Port      int
}

// Bookmark is a per-user quick-access pointer to a resource (Feature 24).
type Bookmark struct {
	ID           string
	UserID       string
	Label        string
	ResourceType string
	ResourceRef  string
	GroupName    string
	OrderIndex   int
	CreatedAt    time.Time
}

// ResourceSample is one 15s-polled CPU/RAM/disk-IO reading for a container
// (Feature 9). DiskRead/WriteBytes are cumulative since container start.
type ResourceSample struct {
	ID             string
	ContainerID    string
	NodeID         string
	CPUPct         float64
	MemBytes       int64
	MemLimitBytes  int64
	DiskReadBytes  int64
	DiskWriteBytes int64
	SampledAt      time.Time
}

// VolumeSample is one daily size/refcount reading for a volume (Feature 7).
type VolumeSample struct {
	ID         string
	NodeID     string
	VolumeName string
	SizeBytes  int64
	RefCount   int64
	SampledAt  time.Time
}

// SecurityAck suppresses a specific flag's badge/alert.
type SecurityAck struct {
	ID             string
	NodeID         string
	ContainerID    string
	FlagType       string
	FlagKey        string
	AcknowledgedBy *string
	Note           string
	CreatedAt      time.Time
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
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUserRole(ctx context.Context, id, role string) error
	DeleteUser(ctx context.Context, id string) error
	CountUsersByRole(ctx context.Context, role string) (int, error)

	// Sessions
	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	RevokeSession(ctx context.Context, id string, at time.Time) error
	ListSessionsByUser(ctx context.Context, userID string) ([]Session, error)
	RevokeAllUserSessions(ctx context.Context, userID string, at time.Time) error

	// Activity (append-only; no update/delete by design)
	AppendActivity(ctx context.Context, e ActivityEntry) error
	ListActivity(ctx context.Context, f ActivityFilter) ([]ActivityEntry, error)
	QueryActivityLog(ctx context.Context, q ActivityQuery) ([]ActivityEntry, error)

	// Nodes
	CreateNode(ctx context.Context, n Node) error
	GetNode(ctx context.Context, id string) (Node, error)
	ListNodes(ctx context.Context) ([]Node, error)
	UpdateNode(ctx context.Context, n Node) error
	DeleteNode(ctx context.Context, id string) error

	// Inventory: VMs (proxmox guests)
	UpsertVM(ctx context.Context, v VM) error
	ListVMsByNode(ctx context.Context, nodeID string) ([]VM, error)
	DeleteVM(ctx context.Context, id string) error

	// Inventory: containers
	UpsertContainer(ctx context.Context, c Container) error
	ListContainersByNode(ctx context.Context, nodeID string) ([]Container, error)
	GetContainer(ctx context.Context, id string) (Container, error)
	DeleteContainer(ctx context.Context, id string) error

	// Mounts (bind-mount index)
	ReplaceContainerMounts(ctx context.Context, containerID string, rows []MountRow) error
	ListMountsByNode(ctx context.Context, nodeID string) ([]MountRow, error)
	ListMountsByContainer(ctx context.Context, containerID string) ([]MountRow, error)

	// Security (Feature 18/19)
	UpsertContainerSecurity(ctx context.Context, row ContainerSecurityRow) error
	GetContainerSecurity(ctx context.Context, containerID string) (ContainerSecurityRow, error)
	ListContainerSecurity(ctx context.Context) ([]ContainerSecurityRow, error)
	SetPortExposures(ctx context.Context, containerID string, rows []PortExposureRow) error
	ListPortExposuresByContainer(ctx context.Context, containerID string) ([]PortExposureRow, error)
	ListAllPortExposures(ctx context.Context) ([]PortExposureRow, error)
	InsertAck(ctx context.Context, a SecurityAck) error
	DeleteAck(ctx context.Context, id string) error
	ListAcks(ctx context.Context) ([]SecurityAck, error)

	// Volume health (Feature 7) — size-trend samples
	InsertVolumeSample(ctx context.Context, s VolumeSample) error
	ListVolumeSamplesByNode(ctx context.Context, nodeID string) ([]VolumeSample, error)
	PruneVolumeSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error)

	// Resource timeline (Feature 9) — per-container metric samples
	InsertResourceSample(ctx context.Context, s ResourceSample) error
	ListResourceSamples(ctx context.Context, containerID string, from, to time.Time) ([]ResourceSample, error)
	PruneResourceSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error)

	// Wake-on-LAN (Feature 6) — per-node config
	UpsertWOLConfig(ctx context.Context, c WOLConfig) error
	GetWOLConfig(ctx context.Context, nodeID string) (WOLConfig, error)

	// Image updates (Feature 15, detection)
	UpsertImageUpdate(ctx context.Context, row ImageUpdateRow) error
	ListImageUpdates(ctx context.Context) ([]ImageUpdateRow, error)

	// CVE scans (Feature 20)
	UpsertImageScan(ctx context.Context, row ImageScanRow) error
	ListImageScans(ctx context.Context) ([]ImageScanRow, error)
	GetImageScan(ctx context.Context, imageDigest string) (ImageScanRow, error)
	ReplaceCVEResults(ctx context.Context, imageDigest string, rows []CVEResultRow) error
	ListCVEResults(ctx context.Context, imageDigest string) ([]CVEResultRow, error)

	// TOTP 2FA (Feature 7)
	UpsertUserTOTP(ctx context.Context, t UserTOTP) error
	GetUserTOTP(ctx context.Context, userID string) (UserTOTP, error)
	DeleteUserTOTP(ctx context.Context, userID string) error

	// Backups (Feature 28)
	CreateBackup(ctx context.Context, b BackupRow) error
	UpdateBackup(ctx context.Context, b BackupRow) error
	ListBackups(ctx context.Context) ([]BackupRow, error)

	// Script runner (Feature 27)
	CreateScript(ctx context.Context, s Script) error
	ListScripts(ctx context.Context) ([]Script, error)
	GetScript(ctx context.Context, id string) (Script, error)
	UpdateScript(ctx context.Context, s Script) error
	DeleteScript(ctx context.Context, id string) error

	// Secrets vault (Feature 12)
	CreateSecretGroup(ctx context.Context, g SecretGroup) error
	ListSecretGroups(ctx context.Context) ([]SecretGroup, error)
	DeleteSecretGroup(ctx context.Context, id string) error
	UpsertSecret(ctx context.Context, s SecretRow) error
	ListSecretsByGroup(ctx context.Context, groupID string) ([]SecretRow, error)
	// ListSecretKeysByGroup returns only id+key (never the encrypted blob), so a
	// listing can't accidentally carry secret material into a response.
	ListSecretKeysByGroup(ctx context.Context, groupID string) ([]SecretRow, error)
	GetSecret(ctx context.Context, id string) (SecretRow, error)
	DeleteSecret(ctx context.Context, id string) error

	// Templates (Feature 14)
	CreateTemplate(ctx context.Context, t Template) error
	ListTemplates(ctx context.Context) ([]Template, error)
	GetTemplate(ctx context.Context, id string) (Template, error)
	UpdateTemplate(ctx context.Context, t Template) error
	DeleteTemplate(ctx context.Context, id string) error
	AddTemplateVersion(ctx context.Context, id string, v TemplateVersion) error
	ListTemplateVersions(ctx context.Context, id string) ([]TemplateVersion, error)

	// Notification webhooks (Feature 26)
	CreateWebhook(ctx context.Context, c WebhookConfig) error
	ListWebhooks(ctx context.Context) ([]WebhookConfig, error)
	GetWebhook(ctx context.Context, id string) (WebhookConfig, error)
	UpdateWebhook(ctx context.Context, c WebhookConfig) error
	DeleteWebhook(ctx context.Context, id string) error

	// Bookmarks (Feature 24) — per-user; mutations scoped by user_id
	CreateBookmark(ctx context.Context, b Bookmark) error
	ListBookmarksByUser(ctx context.Context, userID string) ([]Bookmark, error)
	DeleteBookmark(ctx context.Context, id, userID string) error
	SetBookmarkOrder(ctx context.Context, userID string, orderedIDs []string) error

	Close() error
}

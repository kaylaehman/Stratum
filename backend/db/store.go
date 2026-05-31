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
	// LinkedVMID manually correlates a standalone Docker node to the Proxmox
	// guest it runs as. Tri-state: nil = AUTO (match by name), *0 = NONE
	// (force-unlinked), *>=100 = explicit Proxmox VMID.
	LinkedVMID *int
	Status     string
	LastError  string
	LastSeen   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
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

// CveSchedule is a recurring CVE scan schedule (Feature 20 extension).
// TargetType is "node" (all containers on the node) or "container" (single
// container). IntervalSeconds is the minimum time between scans.
type CveSchedule struct {
	ID              string     `json:"id"`
	TargetType      string     `json:"target_type"`
	TargetID        string     `json:"target_id"`
	Label           string     `json:"label"`
	IntervalSeconds int        `json:"interval_seconds"`
	Enabled         bool       `json:"enabled"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
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
	// UnknownReason is a human-readable explanation of why the status is
	// "unknown" (e.g. "local digest unavailable", "registry lookup failed:
	// 401 Unauthorized"). Empty when status != "unknown".
	UnknownReason string
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

// Snapshot is a rollback checkpoint of a container's full create-spec
// (Feature 15/17). SpecJSON is docker.RecreateSpec marshaled. Listed by
// (NodeID, ContainerName) since a recreate changes the container's docker id.
type Snapshot struct {
	ID            string
	ContainerID   string
	NodeID        string
	ContainerName string
	Reason        string
	ImageRef      string
	ImageDigest   string
	SpecJSON      string
	CreatedAt     time.Time
}

// ProxyConfig is a node's reverse-proxy admin endpoint + sealed API token
// (Feature F1). TokenEncrypted is AES-sealed; the plaintext is never returned.
type ProxyConfig struct {
	NodeID         string
	Endpoint       string
	TokenEncrypted []byte
	UpdatedAt      time.Time
}

// DNSConfig is a node's DNS admin endpoint + sealed API token (Feature F3).
type DNSConfig struct {
	NodeID         string
	Endpoint       string
	TokenEncrypted []byte
	UpdatedAt      time.Time
}

// ChatConfig is the inbound chat-bot config (Feature F8), a single row. The bot
// token is AES-sealed; AllowedChats are the authorized chat IDs.
type ChatConfig struct {
	Provider       string
	TokenEncrypted []byte
	AllowedChats   []int64
	UpdatedAt      time.Time
}

// Runbook is a saved diagnostic/remediation procedure the AI can reference
// (Feature F9). TriggerConditions and Steps are free-text lists.
type Runbook struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TriggerConditions []string  `json:"trigger_conditions"`
	Steps             []string  `json:"steps"`
	RequiresApproval  bool      `json:"requires_approval"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// UptimeMonitor is a configured endpoint health check (Feature: uptime monitoring).
// NodeID is nil for backend-originated checks.
type UptimeMonitor struct {
	ID              string
	Name            string
	Type            string // http | tcp | icmp
	Target          string
	IntervalSeconds int
	TimeoutMs       int
	Expected        string
	Enabled         bool
	NodeID          *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UptimeResult is one recorded outcome of an uptime check.
type UptimeResult struct {
	ID             string
	MonitorID      string
	CheckedAt      time.Time
	Status         string // up | down | degraded
	ResponseTimeMs int
	Error          string
}

// CustomSkill is a user-authored troubleshooting skill, stored as the verbatim
// YAML the operator wrote (parsed into the in-memory library at load time). ID
// is the skill's own id parsed from that YAML.
type CustomSkill struct {
	ID        string    `json:"id"`
	YAML      string    `json:"yaml"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SSOConfig is per-container access-control configuration (Feature F2). It is
// configuration only — the enforcing auth gateway is a follow-on.
// ClientSecretEncrypted is AES-sealed; the plaintext is never returned.
type SSOConfig struct {
	ID                    string    `json:"id"`
	NodeID                string    `json:"node_id"`
	ContainerName         string    `json:"container_name"`
	Enabled               bool      `json:"enabled"`
	Method                string    `json:"method"`
	ProviderURL           string    `json:"provider_url"`
	ClientID              string    `json:"client_id"`
	ClientSecretEncrypted []byte    `json:"-"`
	AllowedGroups         []string  `json:"allowed_groups"`
	SessionDurationSecs   int       `json:"session_duration_secs"`
	HasClientSecret       bool      `json:"has_client_secret"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// FileWatch is a configured path to monitor for changes (Feature 22).
type FileWatch struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Path      string    `json:"path"`
	Recursive bool      `json:"recursive"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// FileEvent is a detected change under a watched path (Feature 22).
type FileEvent struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"node_id"`
	Path       string    `json:"path"`
	EventType  string    `json:"event_type"`
	DetectedAt time.Time `json:"detected_at"`
}

// AgentMemory is a persistent per-scope note the AI assistant uses (Feature F9).
// Scope is "global" | "node" | "container"; ScopeID is "" for global. Source is
// "user" | "ai" | "observed"; AI-proposed memories stay Confirmed=false until a
// user accepts them, and only confirmed memories are injected into AI context.
type AgentMemory struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"`
	ScopeID   string    `json:"scope_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	Confirmed bool      `json:"confirmed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CertInfo is a TLS certificate discovered on a node's filesystem (Feature F4).
// Monitor-only: Stratum surfaces expiry, never issues certs.
type CertInfo struct {
	ID          string     `json:"id"`
	NodeID      string     `json:"node_id"`
	Source      string     `json:"source"`
	Domain      string     `json:"domain"`
	SANs        []string   `json:"sans"`
	Issuer      string     `json:"issuer"`
	Path        string     `json:"path"`
	NotBefore   *time.Time `json:"not_before,omitempty"`
	NotAfter    *time.Time `json:"not_after,omitempty"`
	LastChecked time.Time  `json:"last_checked"`
}

// AIConfig is the single-row AI Assistant provider configuration (Feature 31).
// APIKeyEncrypted is an AES-sealed blob; the plaintext key is never stored.
type AIConfig struct {
	Provider        string
	OllamaBaseURL   string
	OllamaModel     string
	ClaudeModel     string
	OpenAIModel     string
	OpenAIBaseURL   string
	GeminiModel     string
	APIKeyEncrypted []byte
	// Claude OAuth ("-p" method, Feature 31): AES-sealed access/refresh tokens
	// and the access-token expiry. Empty unless the operator connected via OAuth.
	OAuthAccessEncrypted  []byte
	OAuthRefreshEncrypted []byte
	OAuthExpiresAt        time.Time
	UpdatedAt             time.Time
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

// RemediationProposal is a single agentic-remediation proposal generated from a
// diagnostic result, runbook, or AI suggestion. The lifecycle is strictly
// linear: proposed → approved/rejected → executed/failed. Commands contains
// the exact shell commands to run on the target node; they are never run
// without an explicit approval action.
type RemediationProposal struct {
	ID          string     `json:"id"`
	Source      string     `json:"source"`       // diagnostic | runbook | ai
	Title       string     `json:"title"`
	Rationale   string     `json:"rationale"`
	NodeID      string     `json:"node_id"`
	ContainerID string     `json:"container_id,omitempty"`
	Commands    []string   `json:"commands"`
	RiskLevel   string     `json:"risk_level"`   // low | medium | high | destructive
	Status      string     `json:"status"`        // proposed | approved | rejected | executed | failed
	CreatedBy   string     `json:"created_by"`
	ApprovedBy  string     `json:"approved_by,omitempty"`
	Stdout      string     `json:"stdout,omitempty"`
	Stderr      string     `json:"stderr,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	ExecutedAt  *time.Time `json:"executed_at,omitempty"`
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
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	UpdateUserProfile(ctx context.Context, id, username, email string) error
	DeleteUser(ctx context.Context, id string) error
	CountUsersByRole(ctx context.Context, role string) (int, error)

	// Sessions
	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	RevokeSession(ctx context.Context, id string, at time.Time) error
	ListSessionsByUser(ctx context.Context, userID string) ([]Session, error)
	RevokeAllUserSessions(ctx context.Context, userID string, at time.Time) error
	DeleteExpiredSessionsByUser(ctx context.Context, userID string, now time.Time) error

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

	// CVE schedules (Feature 20 extension — recurring scans)
	CreateCveSchedule(ctx context.Context, s CveSchedule) error
	ListCveSchedules(ctx context.Context) ([]CveSchedule, error)
	GetCveSchedule(ctx context.Context, id string) (CveSchedule, error)
	UpdateCveScheduleEnabled(ctx context.Context, id string, enabled bool) error
	UpdateCveScheduleLastRun(ctx context.Context, id string, t time.Time) error
	DeleteCveSchedule(ctx context.Context, id string) error

	// TOTP 2FA (Feature 7)
	UpsertUserTOTP(ctx context.Context, t UserTOTP) error
	GetUserTOTP(ctx context.Context, userID string) (UserTOTP, error)
	DeleteUserTOTP(ctx context.Context, userID string) error

	// Backups (Feature 28)
	CreateBackup(ctx context.Context, b BackupRow) error
	UpdateBackup(ctx context.Context, b BackupRow) error
	ListBackups(ctx context.Context) ([]BackupRow, error)

	// Snapshots / rollback (Feature 15/17)
	CreateSnapshot(ctx context.Context, s Snapshot) error
	GetSnapshot(ctx context.Context, id string) (Snapshot, error)
	DeleteSnapshot(ctx context.Context, id string) error
	ListSnapshotsByContainer(ctx context.Context, nodeID, containerName string) ([]Snapshot, error)
	// PruneSnapshots keeps the newest `keep` snapshots for (nodeID, containerName)
	// and deletes the rest.
	PruneSnapshots(ctx context.Context, nodeID, containerName string, keep int) error

	// AI Assistant config (Feature 31) — single row
	GetAIConfig(ctx context.Context) (AIConfig, error)
	UpsertAIConfig(ctx context.Context, c AIConfig) error

	// Certificates (Feature F4) — replaced wholesale per node on scan
	ReplaceCertsByNode(ctx context.Context, nodeID string, certs []CertInfo) error
	ListCerts(ctx context.Context) ([]CertInfo, error)

	// Reverse-proxy per-node admin config (Feature F1)
	GetProxyConfig(ctx context.Context, nodeID string) (ProxyConfig, error)
	UpsertProxyConfig(ctx context.Context, c ProxyConfig) error

	// DNS per-node admin config (Feature F3)
	GetDNSConfig(ctx context.Context, nodeID string) (DNSConfig, error)
	UpsertDNSConfig(ctx context.Context, c DNSConfig) error

	// Feature flags (FEATURES.md) — only explicitly-set rows are stored
	ListFeatureFlags(ctx context.Context) (map[string]bool, error)
	SetFeatureFlag(ctx context.Context, key string, enabled bool) error

	// Chat bot config (Feature F8) — single row
	GetChatConfig(ctx context.Context) (ChatConfig, error)
	UpsertChatConfig(ctx context.Context, c ChatConfig) error

	// SSO passthrough config (Feature F2) — per (node, container)
	ListSSOConfigs(ctx context.Context) ([]SSOConfig, error)
	UpsertSSOConfig(ctx context.Context, c SSOConfig) (SSOConfig, error)
	DeleteSSOConfig(ctx context.Context, id string) error

	// File change detection (Feature 22)
	CreateFileWatch(ctx context.Context, w FileWatch) error
	ListFileWatchesByNode(ctx context.Context, nodeID string) ([]FileWatch, error)
	DeleteFileWatch(ctx context.Context, id string) error
	InsertFileEvent(ctx context.Context, e FileEvent) error
	ListFileEvents(ctx context.Context, nodeID string, limit int) ([]FileEvent, error)

	// Runbooks (Feature F9)
	CreateRunbook(ctx context.Context, rb Runbook) error
	GetRunbook(ctx context.Context, id string) (Runbook, error)
	ListRunbooks(ctx context.Context) ([]Runbook, error)
	UpdateRunbook(ctx context.Context, rb Runbook) error
	DeleteRunbook(ctx context.Context, id string) error

	// Custom (user-authored) skills — editable counterpart to the built-in
	// container-troubleshooting library. The full YAML is stored verbatim.
	UpsertCustomSkill(ctx context.Context, cs CustomSkill) error
	GetCustomSkill(ctx context.Context, id string) (CustomSkill, error)
	ListCustomSkills(ctx context.Context) ([]CustomSkill, error)
	DeleteCustomSkill(ctx context.Context, id string) error

	// Agent memory (Feature F9)
	CreateAgentMemory(ctx context.Context, m AgentMemory) error
	GetAgentMemory(ctx context.Context, id string) (AgentMemory, error)
	UpdateAgentMemory(ctx context.Context, m AgentMemory) error
	DeleteAgentMemory(ctx context.Context, id string) error
	// ListAgentMemory returns memories for a scope+id; if confirmedOnly, only
	// accepted ones (used to build AI context).
	ListAgentMemory(ctx context.Context, scope, scopeID string, confirmedOnly bool) ([]AgentMemory, error)

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

	// Uptime monitors + results
	CreateUptimeMonitor(ctx context.Context, m UptimeMonitor) error
	GetUptimeMonitor(ctx context.Context, id string) (UptimeMonitor, error)
	ListUptimeMonitors(ctx context.Context) ([]UptimeMonitor, error)
	UpdateUptimeMonitor(ctx context.Context, m UptimeMonitor) error
	DeleteUptimeMonitor(ctx context.Context, id string) error
	InsertUptimeResult(ctx context.Context, r UptimeResult) error
	ListUptimeResults(ctx context.Context, monitorID string, from, to time.Time) ([]UptimeResult, error)
	LatestUptimeResult(ctx context.Context, monitorID string) (UptimeResult, error)
	PruneUptimeResultsBefore(ctx context.Context, cutoff time.Time) (int64, error)

	// Agentic remediation proposals
	CreateProposal(ctx context.Context, p RemediationProposal) error
	GetProposal(ctx context.Context, id string) (RemediationProposal, error)
	ListProposals(ctx context.Context, nodeID string) ([]RemediationProposal, error)
	UpdateProposalStatus(ctx context.Context, id, status, approvedBy string) error
	UpdateProposalExecution(ctx context.Context, id, status, stdout, stderr string, exitCode int) error

	Close() error
}

package api

import (
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/ai"
	"github.com/KAE-Labs/stratum/backend/alertpolicy"
	"github.com/KAE-Labs/stratum/backend/auth"
	"github.com/KAE-Labs/stratum/backend/agentinstall"
	"github.com/KAE-Labs/stratum/backend/automation"
	"github.com/KAE-Labs/stratum/backend/backup"
	"github.com/KAE-Labs/stratum/backend/certs"
	"github.com/KAE-Labs/stratum/backend/chatbot"
	"github.com/KAE-Labs/stratum/backend/configversion"
	"github.com/KAE-Labs/stratum/backend/cve"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/depgraph"
	dnspkg "github.com/KAE-Labs/stratum/backend/dns"
	"github.com/KAE-Labs/stratum/backend/drexport"
	"github.com/KAE-Labs/stratum/backend/features"
	"github.com/KAE-Labs/stratum/backend/placement"
	"github.com/KAE-Labs/stratum/backend/push"
	"github.com/KAE-Labs/stratum/backend/filewatch"
	"github.com/KAE-Labs/stratum/backend/forecast"
	"github.com/KAE-Labs/stratum/backend/fs"
	"github.com/KAE-Labs/stratum/backend/hub"
	"github.com/KAE-Labs/stratum/backend/inventory"
	"github.com/KAE-Labs/stratum/backend/logtail"
	"github.com/KAE-Labs/stratum/backend/mountindex"
	"github.com/KAE-Labs/stratum/backend/nodeconn"
	"github.com/KAE-Labs/stratum/backend/nodes"
	"github.com/KAE-Labs/stratum/backend/orchestration"
	"github.com/KAE-Labs/stratum/backend/permissions"
	"github.com/KAE-Labs/stratum/backend/proxy"
	"github.com/KAE-Labs/stratum/backend/recreate"
	"github.com/KAE-Labs/stratum/backend/remediation"
	"github.com/KAE-Labs/stratum/backend/scheduler"
	"github.com/KAE-Labs/stratum/backend/secrets"
	"github.com/KAE-Labs/stratum/backend/stacks"
	"github.com/KAE-Labs/stratum/backend/security"
	"github.com/KAE-Labs/stratum/backend/skills"
	"github.com/KAE-Labs/stratum/backend/sso"
	"github.com/KAE-Labs/stratum/backend/topology"
	"github.com/KAE-Labs/stratum/backend/twofa"
	"github.com/KAE-Labs/stratum/backend/updates"
	"github.com/KAE-Labs/stratum/backend/uptime"
	"github.com/KAE-Labs/stratum/backend/volumes"
	"github.com/KAE-Labs/stratum/backend/webhooks"
)

// Handlers carries the dependencies shared by all HTTP handlers.
type Handlers struct {
	Store          db.Store
	Activity       *activity.Store
	JWT            *auth.JWT
	Hub            *hub.Hub
	Nodes          *nodes.Service
	Poller         *inventory.Poller
	Files          *fs.Service
	Conn           *nodeconn.Manager
	ContainerUsers *permissions.ContainerCache
	Logs           *logtail.Manager
	Mounts         *mountindex.Index
	Security       *security.Scanner
	Volumes        *volumes.Service
	Topology       *topology.Service
	DepGraph       *depgraph.Service
	Webhooks       *webhooks.Dispatcher
	Updater        *updates.Service
	Secrets        *secrets.Service
	Scheduler      *scheduler.Service
	CVE            *cve.Service
	Backups        *backup.Service
	TwoFA          *twofa.Service
	Recreate       *recreate.Service
	Stacks         *stacks.Service
	AI             *ai.Service
	Remediation    *remediation.Service
	Certs          *certs.Service
	AgentInstall   *agentinstall.Service
	Proxy          *proxy.Service
	DNS            *dnspkg.Service
	Features       *features.Service
	Chat           *chatbot.Service
	FileWatch      *filewatch.Service
	SSO            *sso.Service
	Skills         *skills.Library
	Uptime         *uptime.Service
	Automation     *automation.Engine
	Orchestration  *orchestration.Service
	ConfigVersions *configversion.Service
	Forecast       *forecast.Service
	AlertPolicy    *alertpolicy.Service
	DRExportSvc    *drexport.Service
	Placement      *placement.Service
	Push           *push.Service
	Logger         *slog.Logger
	StartedAt      time.Time

	// SecureCookies controls the Secure attribute on the refresh cookie.
	// Set false for plain-HTTP local dev, true behind TLS.
	SecureCookies bool

	// PreviewLimiter throttles the SSRF-adjacent probe-preview endpoint.
	PreviewLimiter *rate.Limiter

	// LoginLimiter throttles the unauthenticated login endpoint per client IP
	// (brute-force / credential-stuffing defense). AIAskLimiter throttles the
	// expensive external-LLM egress endpoint per client IP. Both fail open when nil.
	LoginLimiter *keyedLimiter
	AIAskLimiter *keyedLimiter
	// AgentTokenLimiter throttles the token-authed agent enroll/binary endpoints
	// per client IP (enroll is a cert-issuance oracle; binary is a bandwidth sink).
	AgentTokenLimiter *keyedLimiter

	// userMu serialises admin-count-sensitive user mutations (role change,
	// delete) so two concurrent demotions can't both pass the last-admin guard
	// and leave zero admins. Single-process SQLite deployment, so an in-process
	// mutex is sufficient. Zero value is ready to use.
	userMu sync.Mutex
}

func ptr(s string) *string { return &s }

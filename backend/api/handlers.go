package api

import (
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/ai"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/automation"
	"github.com/kaylaehman/stratum/backend/backup"
	"github.com/kaylaehman/stratum/backend/certs"
	"github.com/kaylaehman/stratum/backend/chatbot"
	"github.com/kaylaehman/stratum/backend/cve"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/depgraph"
	dnspkg "github.com/kaylaehman/stratum/backend/dns"
	"github.com/kaylaehman/stratum/backend/features"
	"github.com/kaylaehman/stratum/backend/filewatch"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/inventory"
	"github.com/kaylaehman/stratum/backend/logtail"
	"github.com/kaylaehman/stratum/backend/mountindex"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/permissions"
	"github.com/kaylaehman/stratum/backend/proxy"
	"github.com/kaylaehman/stratum/backend/recreate"
	"github.com/kaylaehman/stratum/backend/remediation"
	"github.com/kaylaehman/stratum/backend/scheduler"
	"github.com/kaylaehman/stratum/backend/secrets"
	"github.com/kaylaehman/stratum/backend/stacks"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/skills"
	"github.com/kaylaehman/stratum/backend/sso"
	"github.com/kaylaehman/stratum/backend/topology"
	"github.com/kaylaehman/stratum/backend/twofa"
	"github.com/kaylaehman/stratum/backend/updates"
	"github.com/kaylaehman/stratum/backend/uptime"
	"github.com/kaylaehman/stratum/backend/volumes"
	"github.com/kaylaehman/stratum/backend/webhooks"
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
	Proxy          *proxy.Service
	DNS            *dnspkg.Service
	Features       *features.Service
	Chat           *chatbot.Service
	FileWatch      *filewatch.Service
	SSO            *sso.Service
	Skills         *skills.Library
	Uptime         *uptime.Service
	Automation     *automation.Engine
	Logger         *slog.Logger
	StartedAt      time.Time

	// SecureCookies controls the Secure attribute on the refresh cookie.
	// Set false for plain-HTTP local dev, true behind TLS.
	SecureCookies bool

	// PreviewLimiter throttles the SSRF-adjacent probe-preview endpoint.
	PreviewLimiter *rate.Limiter

	// userMu serialises admin-count-sensitive user mutations (role change,
	// delete) so two concurrent demotions can't both pass the last-admin guard
	// and leave zero admins. Single-process SQLite deployment, so an in-process
	// mutex is sufficient. Zero value is ready to use.
	userMu sync.Mutex
}

func ptr(s string) *string { return &s }

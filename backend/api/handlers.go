package api

import (
	"log/slog"
	"time"

	"golang.org/x/time/rate"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/depgraph"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/inventory"
	"github.com/kaylaehman/stratum/backend/logtail"
	"github.com/kaylaehman/stratum/backend/mountindex"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/permissions"
	"github.com/kaylaehman/stratum/backend/scheduler"
	"github.com/kaylaehman/stratum/backend/secrets"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/topology"
	"github.com/kaylaehman/stratum/backend/updates"
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
	Logger         *slog.Logger
	StartedAt      time.Time

	// SecureCookies controls the Secure attribute on the refresh cookie.
	// Set false for plain-HTTP local dev, true behind TLS.
	SecureCookies bool

	// PreviewLimiter throttles the SSRF-adjacent probe-preview endpoint.
	PreviewLimiter *rate.Limiter
}

func ptr(s string) *string { return &s }

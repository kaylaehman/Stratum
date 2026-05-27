package api

import (
	"log/slog"
	"time"

	"golang.org/x/time/rate"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/inventory"
	"github.com/kaylaehman/stratum/backend/logtail"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/permissions"
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
	Logger         *slog.Logger
	StartedAt      time.Time

	// SecureCookies controls the Secure attribute on the refresh cookie.
	// Set false for plain-HTTP local dev, true behind TLS.
	SecureCookies bool

	// PreviewLimiter throttles the SSRF-adjacent probe-preview endpoint.
	PreviewLimiter *rate.Limiter
}

func ptr(s string) *string { return &s }

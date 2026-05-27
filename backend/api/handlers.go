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
	"github.com/kaylaehman/stratum/backend/nodes"
)

// Handlers carries the dependencies shared by all HTTP handlers.
type Handlers struct {
	Store     db.Store
	Activity  *activity.Store
	JWT       *auth.JWT
	Hub       *hub.Hub
	Nodes     *nodes.Service
	Poller    *inventory.Poller
	Files     *fs.Service
	Logger    *slog.Logger
	StartedAt time.Time

	// SecureCookies controls the Secure attribute on the refresh cookie.
	// Set false for plain-HTTP local dev, true behind TLS.
	SecureCookies bool

	// PreviewLimiter throttles the SSRF-adjacent probe-preview endpoint.
	PreviewLimiter *rate.Limiter
}

func ptr(s string) *string { return &s }

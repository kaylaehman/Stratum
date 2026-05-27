package api

import (
	"log/slog"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/hub"
)

// Handlers carries the dependencies shared by all HTTP handlers.
type Handlers struct {
	Store     db.Store
	Activity  *activity.Store
	JWT       *auth.JWT
	Hub       *hub.Hub
	Logger    *slog.Logger
	StartedAt time.Time

	// SecureCookies controls the Secure attribute on the refresh cookie.
	// Set false for plain-HTTP local dev, true behind TLS.
	SecureCookies bool
}

func ptr(s string) *string { return &s }

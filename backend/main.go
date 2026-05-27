// Command stratum is the Stratum backend API server. It loads configuration,
// opens and migrates the database, wires the HTTP router + middleware, and
// serves until interrupted.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/config"
	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/server"
)

// accessTokenTTL is the lifetime of issued access tokens.
const accessTokenTTL = 15 * time.Minute

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	sqldb, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if err := db.Migrate(sqldb); err != nil {
		return err
	}
	store := sqlite.New(sqldb)
	defer store.Close()

	maybeSeedAdmin(context.Background(), store, cfg, logger)

	cipher, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		return err
	}

	jwt := auth.NewJWT(cfg.JWTSecret, accessTokenTTL)
	handlers := &api.Handlers{
		Store:          store,
		Activity:       activity.NewStore(store),
		JWT:            jwt,
		Hub:            hub.New(),
		Nodes:          nodes.NewService(store, cipher),
		Logger:         logger,
		StartedAt:      time.Now(),
		SecureCookies:  strings.HasPrefix(cfg.BaseURL, "https"),
		PreviewLimiter: rate.NewLimiter(rate.Every(2*time.Second), 5),
	}

	router := server.NewRouter(&server.Deps{Handlers: handlers, JWT: jwt, Store: store})
	srv := server.New(fmt.Sprintf(":%d", cfg.Port), router, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return srv.Run(ctx)
}

// maybeSeedAdmin creates the first admin from STRATUM_ADMIN_USER/PASSWORD when
// the users table is empty (a CI/automation escape hatch). The password is
// cleared from the in-memory config afterward; operators should unset the env
// vars after first boot.
func maybeSeedAdmin(ctx context.Context, store db.Store, cfg *config.Config, logger *slog.Logger) {
	if cfg.AdminUser == "" || cfg.AdminPassword == "" {
		return
	}
	n, err := store.CountUsers(ctx)
	if err != nil || n > 0 {
		cfg.AdminPassword = ""
		return
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	cfg.AdminPassword = ""
	if err != nil {
		logger.Error("seed admin: hash", "error", err)
		return
	}
	if err := store.CreateUser(ctx, db.User{
		ID:           uuid.NewString(),
		Username:     cfg.AdminUser,
		PasswordHash: hash,
		Role:         "admin",
	}); err != nil {
		logger.Error("seed admin: create", "error", err)
		return
	}
	logger.Info("seeded admin user from environment", "username", cfg.AdminUser)
}

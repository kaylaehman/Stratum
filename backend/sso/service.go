// Package sso implements the configuration layer for SSO Passthrough
// (Feature F2): per-container access-control settings (method, OIDC provider,
// allowed groups, session duration). It is CONFIGURATION ONLY — the enforcing
// auth gateway (a reverse proxy that sits in front of the container port and
// checks the session before forwarding) is a follow-on that will consume these
// rows via the reverse-proxy adapter layer's forward-auth.
package sso

import (
	"context"
	"errors"
	"net/url"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
)

// ErrInvalid is returned for a malformed config.
var ErrInvalid = errors.New("sso: invalid configuration")

// Service stores per-container SSO config.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
}

// New wires the store + secret cipher.
func New(store db.Store, cipher *crypto.Cipher) *Service {
	return &Service{store: store, cipher: cipher}
}

// List returns all SSO configs (client secrets never included — only HasClientSecret).
func (s *Service) List(ctx context.Context) ([]db.SSOConfig, error) {
	return s.store.ListSSOConfigs(ctx)
}

// Upsert validates and stores a config. clientSecret: nil keeps the existing
// sealed value, "" clears it, a value seals it.
func (s *Service) Upsert(ctx context.Context, in db.SSOConfig, clientSecret *string) (db.SSOConfig, error) {
	if in.NodeID == "" || in.ContainerName == "" {
		return db.SSOConfig{}, ErrInvalid
	}
	switch in.Method {
	case "local", "totp", "oidc", "forward":
	default:
		return db.SSOConfig{}, ErrInvalid
	}
	if (in.Method == "oidc" || in.Method == "forward") && in.ProviderURL != "" {
		if u, err := url.Parse(in.ProviderURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return db.SSOConfig{}, ErrInvalid
		}
	}
	if in.SessionDurationSecs <= 0 {
		in.SessionDurationSecs = 86400
	}
	if in.AllowedGroups == nil {
		in.AllowedGroups = []string{}
	}

	// Resolve the canonical id + preserve the existing secret unless changed.
	existing, _ := s.find(ctx, in.NodeID, in.ContainerName)
	if existing.ID != "" {
		in.ID = existing.ID
		in.ClientSecretEncrypted = existing.ClientSecretEncrypted
	} else {
		in.ID = uuid.NewString()
	}
	if clientSecret != nil {
		if *clientSecret == "" {
			in.ClientSecretEncrypted = nil
		} else {
			sealed, err := s.cipher.Seal([]byte(*clientSecret))
			if err != nil {
				return db.SSOConfig{}, err
			}
			in.ClientSecretEncrypted = sealed
		}
	}
	return s.store.UpsertSSOConfig(ctx, in)
}

// Delete removes a config by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.DeleteSSOConfig(ctx, id)
}

func (s *Service) find(ctx context.Context, nodeID, name string) (db.SSOConfig, error) {
	all, err := s.store.ListSSOConfigs(ctx)
	if err != nil {
		return db.SSOConfig{}, err
	}
	for _, c := range all {
		if c.NodeID == nodeID && c.ContainerName == name {
			return c, nil
		}
	}
	return db.SSOConfig{}, nil
}

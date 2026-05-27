// Package secrets implements the encrypted env-var vault (Feature 12). Values are
// AES-256-GCM sealed at rest via the shared crypto.Cipher (the same primitive
// that protects node credentials) and decrypted only on an explicit, audited
// reveal. Key names are listable without exposing values.
package secrets

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
)

// Service stores and reveals secrets.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
}

// New wires the store and the value cipher.
func New(store db.Store, cipher *crypto.Cipher) *Service {
	return &Service{store: store, cipher: cipher}
}

// SetSecret seals value and upserts it under (groupID, key).
func (s *Service) SetSecret(ctx context.Context, groupID, key, value string) error {
	sealed, err := s.cipher.Seal([]byte(value))
	if err != nil {
		return err
	}
	return s.store.UpsertSecret(ctx, db.SecretRow{
		ID: uuid.NewString(), GroupID: groupID, Key: key, ValueEncrypted: sealed,
	})
}

// Reveal decrypts and returns one secret's key + plaintext value. Callers MUST
// gate this on admin and audit it.
func (s *Service) Reveal(ctx context.Context, id string) (key, value string, err error) {
	row, err := s.store.GetSecret(ctx, id)
	if err != nil {
		return "", "", err
	}
	pt, err := s.cipher.Open(row.ValueEncrypted)
	if err != nil {
		return "", "", err
	}
	return row.Key, string(pt), nil
}

// ImportEnv parses .env text and stores each KEY=VALUE under groupID. Returns the
// number of secrets imported.
func (s *Service) ImportEnv(ctx context.Context, groupID, envText string) (int, error) {
	pairs := ParseEnv(envText)
	for k, v := range pairs {
		if err := s.SetSecret(ctx, groupID, k, v); err != nil {
			return 0, err
		}
	}
	return len(pairs), nil
}

// ParseEnv parses .env-style text into a map. Lines that are blank or start with
// '#' are skipped; each remaining line is split on the first '=' (key trimmed,
// value trimmed and unquoted). Invalid lines (no '=', empty key) are ignored.
func ParseEnv(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Allow a leading "export ".
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = unquote(val)
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

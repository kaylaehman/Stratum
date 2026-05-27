// Package twofa manages per-user TOTP two-factor auth (Feature 7): enrollment,
// enable/disable, login verification, and one-time recovery codes. The TOTP
// secret is AES-sealed at rest (same crypto.Cipher as node creds/secrets) and
// recovery codes are bcrypt-hashed and consumed on use.
package twofa

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/totp"
)

// ErrInvalidCode is returned when a TOTP/recovery code doesn't verify.
var ErrInvalidCode = errors.New("twofa: invalid code")

// now is overridable in tests.
var now = time.Now

const recoveryCount = 8

// Service handles 2FA operations.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
	issuer string

	// locks serialises the read-modify-write of a user's TOTP row so two
	// concurrent logins cannot consume the same recovery code twice, and a
	// re-enrollment cannot race the possession check. Keyed by user ID; this
	// backend runs as a single process (SQLite single binary), so an in-process
	// mutex is sufficient.
	locks sync.Map // userID -> *sync.Mutex

	// grace tracks a per-user step-up confirmation window (Feature F7): after a
	// successful challenge, destructive actions are allowed without re-prompting
	// for StepUpGrace. In-process (single binary); cleared on restart.
	graceMu sync.Mutex
	grace   map[string]time.Time
}

// StepUpGrace is how long a step-up TOTP confirmation is honored before a
// destructive action must be re-challenged.
const StepUpGrace = 5 * time.Minute

// New wires the store + secret cipher.
func New(store db.Store, cipher *crypto.Cipher) *Service {
	return &Service{store: store, cipher: cipher, issuer: "Stratum", grace: map[string]time.Time{}}
}

// ChallengeStepUp verifies a TOTP code (recovery codes are NOT accepted here, to
// avoid burning a single-use code on a routine confirmation) and, on success,
// opens the step-up grace window for the user. Returns ErrInvalidCode on a bad
// code. Callers should only invoke this when Enabled(userID) is true.
func (s *Service) ChallengeStepUp(ctx context.Context, userID, code string) error {
	rec, err := s.store.GetUserTOTP(ctx, userID)
	if err != nil {
		return err
	}
	secret, err := s.openSecret(rec)
	if err != nil {
		return err
	}
	if !totp.Verify(secret, code, now()) {
		return ErrInvalidCode
	}
	s.graceMu.Lock()
	s.grace[userID] = now()
	s.graceMu.Unlock()
	return nil
}

// HasStepUp reports whether the user has a valid (unexpired) step-up grace.
func (s *Service) HasStepUp(userID string) bool {
	s.graceMu.Lock()
	defer s.graceMu.Unlock()
	t, ok := s.grace[userID]
	return ok && now().Sub(t) < StepUpGrace
}

// lock serialises mutations for one user and returns the unlock func.
func (s *Service) lock(userID string) func() {
	mui, _ := s.locks.LoadOrStore(userID, &sync.Mutex{})
	mu := mui.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// SetupResult is returned once at enrollment — the recovery codes are shown
// only here.
type SetupResult struct {
	Secret        string   `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// Setup generates a new (disabled) TOTP secret + recovery codes for a user. The
// plaintext recovery codes are returned once; only their hashes are stored.
//
// If the account already has 2FA *enabled*, re-enrollment would silently
// overwrite the existing secret (and reset Enabled to false) — a session
// hijacker could use that to disable a victim's 2FA without the device. So when
// already enabled, the caller must prove possession via currentCode (TOTP or
// recovery) before the secret is replaced. First-time enrollment passes an
// empty currentCode.
func (s *Service) Setup(ctx context.Context, userID, username, currentCode string) (SetupResult, error) {
	unlock := s.lock(userID)
	defer unlock()

	if existing, err := s.store.GetUserTOTP(ctx, userID); err == nil && existing.Enabled {
		if ok, _ := s.check(existing, currentCode); !ok {
			return SetupResult{}, ErrInvalidCode
		}
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		return SetupResult{}, err
	}
	sealed, err := s.cipher.Seal([]byte(secret))
	if err != nil {
		return SetupResult{}, err
	}
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return SetupResult{}, err
	}
	if err := s.store.UpsertUserTOTP(ctx, db.UserTOTP{
		UserID: userID, SecretEncrypted: sealed, Enabled: false, RecoveryHashes: hashes,
	}); err != nil {
		return SetupResult{}, err
	}
	return SetupResult{
		Secret:          secret,
		ProvisioningURI: totp.ProvisioningURI(secret, username, s.issuer),
		RecoveryCodes:   codes,
	}, nil
}

// Enable activates 2FA after the user proves possession of the secret with a
// valid code.
func (s *Service) Enable(ctx context.Context, userID, code string) error {
	unlock := s.lock(userID)
	defer unlock()
	rec, err := s.store.GetUserTOTP(ctx, userID)
	if err != nil {
		return err
	}
	secret, err := s.openSecret(rec)
	if err != nil {
		return err
	}
	if !totp.Verify(secret, code, now()) {
		return ErrInvalidCode
	}
	rec.Enabled = true
	return s.store.UpsertUserTOTP(ctx, rec)
}

// Disable turns off 2FA after verifying a current code (TOTP or recovery).
func (s *Service) Disable(ctx context.Context, userID, code string) error {
	unlock := s.lock(userID)
	defer unlock()
	rec, err := s.store.GetUserTOTP(ctx, userID)
	if err != nil {
		return err
	}
	ok, _ := s.check(rec, code)
	if !ok {
		return ErrInvalidCode
	}
	return s.store.DeleteUserTOTP(ctx, userID)
}

// Enabled reports whether a user has 2FA active.
func (s *Service) Enabled(ctx context.Context, userID string) bool {
	rec, err := s.store.GetUserTOTP(ctx, userID)
	return err == nil && rec.Enabled
}

// VerifyLogin checks a login code for a user who has 2FA enabled. A recovery
// code is consumed (removed) on successful use. Returns ErrInvalidCode on
// failure. Callers MUST only invoke this when Enabled is true.
//
// Note: a TOTP code stays valid across the ±1-step skew window (~90s) and is
// not single-use within that window — the standard RFC 6238 trade-off. Recovery
// codes, by contrast, are strictly single-use (consumed under the per-user
// lock, so concurrent logins cannot redeem the same code twice).
func (s *Service) VerifyLogin(ctx context.Context, userID, code string) error {
	unlock := s.lock(userID)
	defer unlock()
	rec, err := s.store.GetUserTOTP(ctx, userID)
	if err != nil {
		return err
	}
	ok, consumedIdx := s.check(rec, code)
	if !ok {
		return ErrInvalidCode
	}
	if consumedIdx >= 0 {
		rec.RecoveryHashes = append(rec.RecoveryHashes[:consumedIdx], rec.RecoveryHashes[consumedIdx+1:]...)
		_ = s.store.UpsertUserTOTP(ctx, rec)
	}
	return nil
}

// check verifies a code as either a TOTP code or a recovery code. Returns
// (matched, recoveryIndex) where recoveryIndex >= 0 means a recovery code was
// used (and should be consumed).
func (s *Service) check(rec db.UserTOTP, code string) (bool, int) {
	if secret, err := s.openSecret(rec); err == nil && totp.Verify(secret, code, now()) {
		return true, -1
	}
	for i, h := range rec.RecoveryHashes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(code)) == nil {
			return true, i
		}
	}
	return false, -1
}

func (s *Service) openSecret(rec db.UserTOTP) (string, error) {
	pt, err := s.cipher.Open(rec.SecretEncrypted)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// generateRecoveryCodes returns plaintext codes + their bcrypt hashes.
func generateRecoveryCodes() (codes, hashes []string, err error) {
	for i := 0; i < recoveryCount; i++ {
		c, err := randomCode()
		if err != nil {
			return nil, nil, err
		}
		h, err := bcrypt.GenerateFromPassword([]byte(c), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		codes = append(codes, c)
		hashes = append(hashes, string(h))
	}
	return codes, hashes, nil
}

// randomCode returns a 10-char base32 recovery code.
func randomCode() (string, error) {
	buf := make([]byte, 7) // 7 bytes -> ~11 base32 chars
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)[:10], nil
}

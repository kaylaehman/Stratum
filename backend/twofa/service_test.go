package twofa

import (
	"context"
	"testing"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/totp"
)

// memStore is a minimal db.Store for the 2FA flow.
type memStore struct {
	db.Store
	rec *db.UserTOTP
}

func (m *memStore) UpsertUserTOTP(_ context.Context, t db.UserTOTP) error { c := t; m.rec = &c; return nil }
func (m *memStore) GetUserTOTP(_ context.Context, _ string) (db.UserTOTP, error) {
	if m.rec == nil {
		return db.UserTOTP{}, db.ErrNotFound
	}
	return *m.rec, nil
}
func (m *memStore) DeleteUserTOTP(_ context.Context, _ string) error { m.rec = nil; return nil }

func newSvc(t *testing.T) *Service {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	c, err := crypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	return New(&memStore{}, c)
}

func TestEnrollEnableVerifyFlow(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, err := s.Setup(ctx, "u1", "kayla", "")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if len(res.RecoveryCodes) != recoveryCount || res.Secret == "" {
		t.Fatalf("setup result = %+v", res)
	}
	if s.Enabled(ctx, "u1") {
		t.Error("2FA should not be enabled until confirmed")
	}

	// A wrong code can't enable.
	if err := s.Enable(ctx, "u1", "000000"); err != ErrInvalidCode {
		t.Errorf("enable with bad code = %v, want ErrInvalidCode", err)
	}
	// The correct current code enables.
	code, _ := totp.Code(res.Secret, now())
	if err := s.Enable(ctx, "u1", code); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !s.Enabled(ctx, "u1") {
		t.Error("2FA should be enabled after a valid confirm")
	}

	// Login verify with a TOTP code succeeds.
	code, _ = totp.Code(res.Secret, now())
	if err := s.VerifyLogin(ctx, "u1", code); err != nil {
		t.Errorf("VerifyLogin(totp): %v", err)
	}

	// A recovery code works once, then is consumed.
	rc := res.RecoveryCodes[0]
	if err := s.VerifyLogin(ctx, "u1", rc); err != nil {
		t.Errorf("VerifyLogin(recovery): %v", err)
	}
	if err := s.VerifyLogin(ctx, "u1", rc); err != ErrInvalidCode {
		t.Error("recovery code should be single-use (consumed)")
	}

	// Disable requires a valid code.
	if err := s.Disable(ctx, "u1", "000000"); err != ErrInvalidCode {
		t.Errorf("disable bad code = %v, want ErrInvalidCode", err)
	}
	code, _ = totp.Code(res.Secret, now())
	if err := s.Disable(ctx, "u1", code); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if s.Enabled(ctx, "u1") {
		t.Error("2FA should be disabled after Disable")
	}
}

// TestReenrollRequiresCurrentCode guards the session-hijack hole: once 2FA is
// enabled, a re-Setup without proof of possession must be refused (otherwise it
// would overwrite the secret and silently disable the victim's 2FA).
func TestReenrollRequiresCurrentCode(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, err := s.Setup(ctx, "u1", "kayla", "")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	code, _ := totp.Code(res.Secret, now())
	if err := s.Enable(ctx, "u1", code); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// Re-enroll with no/invalid code is refused, leaving the original intact.
	if _, err := s.Setup(ctx, "u1", "kayla", ""); err != ErrInvalidCode {
		t.Errorf("re-Setup without code = %v, want ErrInvalidCode", err)
	}
	if _, err := s.Setup(ctx, "u1", "kayla", "000000"); err != ErrInvalidCode {
		t.Errorf("re-Setup bad code = %v, want ErrInvalidCode", err)
	}
	if !s.Enabled(ctx, "u1") {
		t.Error("victim's 2FA must remain enabled after a refused re-enrollment")
	}

	// With a valid current code, re-enrollment proceeds (new disabled secret).
	code, _ = totp.Code(res.Secret, now())
	res2, err := s.Setup(ctx, "u1", "kayla", code)
	if err != nil {
		t.Fatalf("re-Setup with valid code: %v", err)
	}
	if res2.Secret == res.Secret {
		t.Error("re-enrollment should mint a fresh secret")
	}
	if s.Enabled(ctx, "u1") {
		t.Error("re-enrollment leaves 2FA disabled until re-confirmed")
	}
}

func TestSecretNotStoredPlaintext(t *testing.T) {
	ctx := context.Background()
	store := &memStore{}
	key := make([]byte, 32)
	c, _ := crypto.New(key)
	s := New(store, c)
	res, _ := s.Setup(ctx, "u1", "kayla", "")
	if string(store.rec.SecretEncrypted) == res.Secret {
		t.Fatal("TOTP secret stored in plaintext")
	}
}

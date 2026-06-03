package twofa

// stepup_failclosed_test.go — edge-case tests for the TOTP step-up gate's
// fail-CLOSED behavior (Bug C5 from the audit reconciliation).
//
// Bug C5: requireStepUp was fail-open — an admin with no TOTP enrolled could
// bypass the step-up gate entirely (Enabled(user) returned false when no TOTP
// row existed, and the old code treated false as "no 2FA needed").
//
// The fix (Wave 1) changed requireStepUp to be fail-CLOSED: absent or
// unenrolled TOTP DENIES the action (returns 428 totp_enrollment_required).
//
// These tests verify the twofa.Service behavior that the API gate relies on:
//  - Enabled returns false for a user with no TOTP row (unenrolled).
//  - Enabled returns false for a user who set up but didn't confirm (not enabled).
//  - HasStepUp returns false for a user with no active grace window.
//  - ChallengeStepUp fails for an unenrolled user (no TOTP row).
//  - ChallengeStepUp fails with ErrInvalidCode for an enrolled user with wrong code.
//  - Only a correct code opens the grace window.
//  - The grace window is per-user (user A's grace does not bleed to user B).

import (
	"context"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/totp"
)

// --- fail-CLOSED: unenrolled users never pass the step-up gate -------------

// TestEnabled_ReturnsFalseWhenUnenrolled is the core premise for fail-CLOSED:
// Enabled must return false when the user has no TOTP row (never enrolled).
// requireStepUp in api/authz.go calls Enabled, then denies if false.
func TestEnabled_ReturnsFalseWhenUnenrolled(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	// "never-enrolled-user" has no TOTP row at all.
	if s.Enabled(ctx, "never-enrolled-user") {
		t.Error("Enabled must return false for a user with no TOTP row (unenrolled)")
	}
}

// TestEnabled_ReturnsFalseWhenSetupButNotEnabled verifies that a user who
// called Setup (got the QR code) but never called Enable (never confirmed) is
// NOT considered enabled — confirming the secret is required.
func TestEnabled_ReturnsFalseWhenSetupButNotEnabled(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	_, err := s.Setup(ctx, "u-partial", "partial", "")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	// At this point the user has a TOTP row but Enabled=false.
	if s.Enabled(ctx, "u-partial") {
		t.Error("Enabled must be false after Setup without Enable (unconfirmed enrollment)")
	}
}

// TestHasStepUp_ReturnsFalseForUnenrolled verifies that HasStepUp returns
// false for a user with no grace window (never challenged successfully).
// This means even if Enabled returned true, HasStepUp gates the actual action.
func TestHasStepUp_ReturnsFalseWithNoWindow(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-nowin", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-nowin", code)

	// Enrolled and enabled, but no challenge yet → no step-up window.
	if s.HasStepUp("u-nowin") {
		t.Error("HasStepUp must return false before any successful challenge")
	}
}

// TestChallengeStepUp_FailsForUnenrolledUser verifies that ChallengeStepUp
// returns an error (not silently succeeds) when the user has no TOTP row.
// This prevents an unenrolled user from calling the challenge endpoint and
// obtaining step-up access without a real second factor.
func TestChallengeStepUp_FailsForUnenrolledUser(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	err := s.ChallengeStepUp(ctx, "unenrolled-user", "123456")
	if err == nil {
		t.Error("ChallengeStepUp should fail for a user with no TOTP row")
	}
	// Must not open a grace window as a side effect.
	if s.HasStepUp("unenrolled-user") {
		t.Error("failed ChallengeStepUp must not open a grace window")
	}
}

// TestChallengeStepUp_WrongCodeDenied verifies that a wrong TOTP code does not
// open the step-up grace window.
func TestChallengeStepUp_WrongCodeDenied(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-wrong", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-wrong", code)

	err := s.ChallengeStepUp(ctx, "u-wrong", "000000")
	if err != ErrInvalidCode {
		t.Errorf("ChallengeStepUp with wrong code = %v; want ErrInvalidCode", err)
	}
	if s.HasStepUp("u-wrong") {
		t.Error("wrong code must not open the step-up window")
	}
}

// TestChallengeStepUp_RecoveryCodeDenied verifies that a recovery code is NOT
// accepted for step-up challenges (only TOTP codes are). Recovery codes are
// single-use and should not be burnable on routine re-authorisations.
func TestChallengeStepUp_RecoveryCodeDenied(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-rc", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-rc", code)

	// Attempt to step up using a real recovery code.
	recoveryCode := res.RecoveryCodes[0]
	err := s.ChallengeStepUp(ctx, "u-rc", recoveryCode)
	if err != ErrInvalidCode {
		t.Errorf("ChallengeStepUp with recovery code = %v; want ErrInvalidCode (recovery codes must not open step-up)", err)
	}
	if s.HasStepUp("u-rc") {
		t.Error("recovery code must not open the step-up window")
	}
}

// TestChallengeStepUp_CorrectCodeOpensWindow verifies the happy path: a valid
// TOTP code opens the grace window for the user.
func TestChallengeStepUp_CorrectCodeOpensWindow(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-happy", "user", "")
	enableCode, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-happy", enableCode)

	challengeCode, _ := totp.Code(res.Secret, now())
	if err := s.ChallengeStepUp(ctx, "u-happy", challengeCode); err != nil {
		t.Fatalf("ChallengeStepUp with correct code: %v", err)
	}
	if !s.HasStepUp("u-happy") {
		t.Error("correct code must open the step-up grace window")
	}
}

// TestStepUpGrace_PerUserIsolation verifies that user A's step-up grace window
// does not bleed into user B's gate check.
func TestStepUpGrace_PerUserIsolation(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	// Enroll user A and challenge successfully.
	resA, _ := s.Setup(ctx, "user-a", "a", "")
	codeA, _ := totp.Code(resA.Secret, now())
	_ = s.Enable(ctx, "user-a", codeA)
	codeA, _ = totp.Code(resA.Secret, now())
	_ = s.ChallengeStepUp(ctx, "user-a", codeA)

	// Enroll user B but do NOT challenge.
	resB, _ := s.Setup(ctx, "user-b", "b", "")
	codeB, _ := totp.Code(resB.Secret, now())
	_ = s.Enable(ctx, "user-b", codeB)

	if !s.HasStepUp("user-a") {
		t.Error("user-a should have step-up grace after a successful challenge")
	}
	if s.HasStepUp("user-b") {
		t.Error("user-b must not have step-up grace (no challenge performed)")
	}
}

// TestStepUpGrace_Expiry verifies that the grace window expires after
// StepUpGrace (Feature F7). Once expired, HasStepUp must return false.
func TestStepUpGrace_ExpiresAfterGracePeriod(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-exp", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-exp", code)
	code, _ = totp.Code(res.Secret, now())
	_ = s.ChallengeStepUp(ctx, "u-exp", code)

	if !s.HasStepUp("u-exp") {
		t.Fatal("step-up should be open immediately after successful challenge")
	}

	// Advance time past the grace window via the now seam.
	orig := now
	now = func() time.Time { return orig().Add(StepUpGrace + time.Second) }
	defer func() { now = orig }()

	if s.HasStepUp("u-exp") {
		t.Errorf("step-up grace should expire after %v; HasStepUp still true", StepUpGrace)
	}
}

// TestEnabled_ReturnsTrueAfterEnableConfirm verifies that once a user has
// enrolled and confirmed their TOTP, Enabled returns true (sanity check
// for the enabling path).
func TestEnabled_ReturnsTrueAfterEnableConfirm(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-confirm", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-confirm", code)

	if !s.Enabled(ctx, "u-confirm") {
		t.Error("Enabled must return true after Setup + Enable with correct code")
	}
}

// TestEnabled_ReturnsFalseAfterDisable verifies that Enabled returns false
// once 2FA has been disabled.
func TestEnabled_ReturnsFalseAfterDisable(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-dis", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-dis", code)
	code, _ = totp.Code(res.Secret, now())
	_ = s.Disable(ctx, "u-dis", code)

	if s.Enabled(ctx, "u-dis") {
		t.Error("Enabled must return false after 2FA is disabled")
	}
}

// TestHasStepUp_RemainsOpenDuringGrace verifies that HasStepUp stays true
// throughout the grace period (not just at the exact moment of challenge).
func TestHasStepUp_RemainsOpenDuringGrace(t *testing.T) {
	ctx := context.Background()
	s := newSvc(t)

	res, _ := s.Setup(ctx, "u-grace", "user", "")
	code, _ := totp.Code(res.Secret, now())
	_ = s.Enable(ctx, "u-grace", code)
	code, _ = totp.Code(res.Secret, now())
	_ = s.ChallengeStepUp(ctx, "u-grace", code)

	// Advance time to just before the grace window expires.
	orig := now
	now = func() time.Time { return orig().Add(StepUpGrace - time.Second) }
	defer func() { now = orig }()

	if !s.HasStepUp("u-grace") {
		t.Error("step-up grace should still be open 1s before expiry")
	}
}

package api

// Tests for requireAdminStepUp (the composed admin+step-up gate) and the
// StepUpPreflight handler. Reuses the reqWithUser / role helpers from
// stepup_gate_internal_test.go.
//
// Coverage note: Handlers.TwoFA is a concrete *twofa.Service that the internal
// harness can't construct (needs a crypto.Cipher), so the two distinct 428
// bodies (totp_enrollment_required vs 2fa_required) are covered in the twofa
// package, not here — the same documented limitation as the sibling gate test.
// What we CAN prove here is the security-critical short-circuit: a non-admin is
// rejected with 403 and requireStepUp never runs.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KAE-Labs/stratum/backend/auth"
	"github.com/KAE-Labs/stratum/backend/db"
)

func TestRequireAdminStepUp_NonAdmin_403_ShortCircuits(t *testing.T) {
	// An operator must be rejected by the admin half with 403 admin_only, and the
	// step-up half must NOT run (else, with nil TwoFA, it would pass and mask the
	// admin rejection, or a real TwoFA would leak enrollment state via a 428).
	h := &Handlers{TwoFA: nil}
	u := db.User{ID: "u1", Role: auth.RoleOperator}
	r := reqWithUser(t, u)
	w := httptest.NewRecorder()

	if h.requireAdminStepUp(w, r) {
		t.Fatal("operator must not pass requireAdminStepUp")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403 (admin gate first, not 428)", w.Code)
	}
}

func TestRequireAdminStepUp_Admin_NilTwoFA_Passes(t *testing.T) {
	// Admin + no 2FA subsystem wired (minimal build) → gate passes.
	h := &Handlers{TwoFA: nil}
	u := db.User{ID: "u1", Role: auth.RoleAdmin}
	r := reqWithUser(t, u)
	w := httptest.NewRecorder()

	if !h.requireAdminStepUp(w, r) {
		t.Errorf("admin with nil TwoFA should pass requireAdminStepUp, got %d", w.Code)
	}
}

func TestStepUpPreflight_Admin_Returns200OK(t *testing.T) {
	h := &Handlers{TwoFA: nil}
	u := db.User{ID: "u1", Role: auth.RoleAdmin}
	r := reqWithUser(t, u)
	w := httptest.NewRecorder()

	h.StepUpPreflight(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	var body struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.OK {
		t.Error("preflight body ok = false; want true")
	}
}

func TestStepUpPreflight_Viewer_403(t *testing.T) {
	h := &Handlers{TwoFA: nil}
	u := db.User{ID: "u1", Role: auth.RoleViewer}
	r := reqWithUser(t, u)
	w := httptest.NewRecorder()

	h.StepUpPreflight(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", w.Code)
	}
}

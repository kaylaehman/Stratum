package api

import (
	"errors"
	"net/http"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/middleware"
	"github.com/kaylaehman/stratum/backend/twofa"
)

// TwoFAStatus reports whether the calling user has 2FA enabled.
func (h *Handlers) TwoFAStatus(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"enabled": h.TwoFA.Enabled(r.Context(), user.ID)})
}

// TwoFASetup begins enrollment: returns the secret, otpauth URI, and one-time
// recovery codes. 2FA is not active until confirmed via Enable. Re-enrolling an
// account that already has 2FA enabled requires proving possession of the
// current factor via current_code (otherwise a hijacked session could reset a
// victim's 2FA); first-time enrollment omits it.
func (h *Handlers) TwoFASetup(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var body struct {
		CurrentCode string `json:"current_code"`
	}
	_ = decodeJSON(r, &body) // body is optional (absent on first enrollment)
	res, err := h.TwoFA.Setup(r.Context(), user.ID, user.Username, body.CurrentCode)
	if errors.Is(err, twofa.ErrInvalidCode) {
		writeError(w, http.StatusBadRequest, "reenroll_requires_code")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "setup_failed")
		return
	}
	auditTwoFA(r, "setup")
	writeJSON(w, http.StatusOK, res)
}

type totpCodeBody struct {
	Code string `json:"code"`
}

// TwoFAEnable activates 2FA after verifying a code. Audited.
func (h *Handlers) TwoFAEnable(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var body totpCodeBody
	if err := decodeJSON(r, &body); err != nil || body.Code == "" {
		writeError(w, http.StatusBadRequest, "code_required")
		return
	}
	if err := h.TwoFA.Enable(r.Context(), user.ID, body.Code); errors.Is(err, twofa.ErrInvalidCode) {
		writeError(w, http.StatusBadRequest, "invalid_code")
		return
	} else if err != nil {
		writeError(w, http.StatusBadRequest, "enable_failed")
		return
	}
	auditTwoFA(r, "enabled")
	w.WriteHeader(http.StatusNoContent)
}

// TwoFADisable turns off 2FA after verifying a code (TOTP or recovery). Audited.
func (h *Handlers) TwoFADisable(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var body totpCodeBody
	if err := decodeJSON(r, &body); err != nil || body.Code == "" {
		writeError(w, http.StatusBadRequest, "code_required")
		return
	}
	if err := h.TwoFA.Disable(r.Context(), user.ID, body.Code); errors.Is(err, twofa.ErrInvalidCode) {
		writeError(w, http.StatusBadRequest, "invalid_code")
		return
	} else if err != nil {
		writeError(w, http.StatusBadRequest, "disable_failed")
		return
	}
	auditTwoFA(r, "disabled")
	w.WriteHeader(http.StatusNoContent)
}

// TwoFAChallenge verifies a TOTP code and opens the step-up grace window so the
// caller can perform a high-risk action (Feature F7). Recovery codes are not
// accepted here (they'd be burned on routine confirmations). Audited.
func (h *Handlers) TwoFAChallenge(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	if !h.TwoFA.Enabled(r.Context(), user.ID) {
		writeError(w, http.StatusBadRequest, "2fa_not_enabled")
		return
	}
	var body totpCodeBody
	if err := decodeJSON(r, &body); err != nil || body.Code == "" {
		writeError(w, http.StatusBadRequest, "code_required")
		return
	}
	if err := h.TwoFA.ChallengeStepUp(r.Context(), user.ID, body.Code); errors.Is(err, twofa.ErrInvalidCode) {
		writeError(w, http.StatusBadRequest, "invalid_code")
		return
	} else if err != nil {
		writeError(w, http.StatusBadRequest, "challenge_failed")
		return
	}
	auditTwoFA(r, "challenge")
	w.WriteHeader(http.StatusNoContent)
}

func auditTwoFA(r *http.Request, state string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "auth.2fa_" + state
		e.TargetType = ptr(activity.TargetUser)
		if u, ok := middleware.UserFromContext(r.Context()); ok {
			e.TargetID = &u.ID
		}
	}
}

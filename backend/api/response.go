// Package api holds the REST + WebSocket HTTP handlers for the Stratum backend.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kaylaehman/stratum/backend/capabilities"
)

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			slog.Error("api: encode response", "error", err)
		}
	}
}

// writeError writes a JSON error body {"error": code}.
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// writeCapabilityError maps an *ErrCapabilityUnavailable to a 422 response with
// the capability name; any other error becomes a generic 500.
func writeCapabilityError(w http.ResponseWriter, err error) {
	var capErr *capabilities.ErrCapabilityUnavailable
	if errors.As(err, &capErr) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error":      "capability_unavailable",
			"capability": string(capErr.Capability),
		})
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error")
}

// decodeJSON reads a JSON request body into v, capping its size.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

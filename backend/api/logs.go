package api

import (
	"errors"
	"net/http"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/logtail"
	"github.com/kaylaehman/stratum/backend/middleware"
)

type logsBody struct {
	ContainerID string `json:"container_id"`
	WSClientID  string `json:"ws_client_id"`
}

// resolveLogContainer validates the request: the WS client must belong to the
// caller, the container must exist, and its node must have docker. Returns the
// container row + the verified client id.
func (h *Handlers) resolveLogContainer(w http.ResponseWriter, r *http.Request) (db.Container, hub.ClientID, bool) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return db.Container{}, "", false
	}
	var body logsBody
	if err := decodeJSON(r, &body); err != nil || body.ContainerID == "" || body.WSClientID == "" {
		writeError(w, http.StatusBadRequest, "container_id_and_ws_client_id_required")
		return db.Container{}, "", false
	}
	clientID := hub.ClientID(body.WSClientID)
	if owner, ok := h.Hub.ClientUser(clientID); !ok || owner != user.ID {
		// The WS connection isn't this user's — never grant it a log topic.
		writeError(w, http.StatusForbidden, "invalid_ws_client")
		return db.Container{}, "", false
	}
	ctr, err := h.Store.GetContainer(r.Context(), body.ContainerID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return db.Container{}, "", false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return db.Container{}, "", false
	}
	return ctr, clientID, true
}

// LogsSubscribe authorizes + starts (refcounted) a container's log tailer and
// grants the caller's WS connection the logs:<dockerID> topic server-side.
func (h *Handlers) LogsSubscribe(w http.ResponseWriter, r *http.Request) {
	ctr, clientID, ok := h.resolveLogContainer(w, r)
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	err := h.Logs.Subscribe(r.Context(), user.ID, ctr.NodeID, ctr.ID, ctr.DockerID, clientID)
	switch {
	case errors.Is(err, logtail.ErrUnauthorized):
		writeError(w, http.StatusForbidden, "unauthorized_container")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "tail_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"topic": "logs:" + ctr.DockerID})
}

// LogsUnsubscribe decrements the tailer refcount for the caller's connection.
func (h *Handlers) LogsUnsubscribe(w http.ResponseWriter, r *http.Request) {
	ctr, clientID, ok := h.resolveLogContainer(w, r)
	if !ok {
		return
	}
	h.Logs.Unsubscribe(ctr.DockerID, clientID)
	w.WriteHeader(http.StatusNoContent)
}

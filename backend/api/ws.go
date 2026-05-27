package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/kaylaehman/stratum/backend/middleware"
)

// clientSubscribablePrefixes are the topic prefixes a WS client may self-join
// via a subscribe message. Sensitive topics (logs:) are NOT here — they are
// granted only server-side after authorization (POST /api/logs/subscribe).
var clientSubscribablePrefixes = []string{"tree:", "ping"}

func clientMaySubscribe(topic string) bool {
	for _, p := range clientSubscribablePrefixes {
		if topic == p || strings.HasPrefix(topic, p) {
			return true
		}
	}
	return false
}

// wsCommand is the client->server control message on the WebSocket. A client
// subscribes to topics (e.g. "tree:<nodeId>") to receive broadcasts.
type wsCommand struct {
	Subscribe   string `json:"subscribe,omitempty"`
	Unsubscribe string `json:"unsubscribe,omitempty"`
}

// WebSocket upgrades the connection, registers it with the hub, and runs a
// trivial ping/pong on the "ping" topic to prove end-to-end fan-out. Feature
// sub-projects (logs, tree) define their own topics on top of this seam.
func (h *Handlers) WebSocket(w http.ResponseWriter, r *http.Request) {
	// Default same-origin enforcement (CSWSH protection): the browser's Origin
	// must match Host. Production serves the SPA same-origin; the Vite dev
	// server proxies /api (incl. /api/ws) to the backend so dev is same-origin
	// too. Non-browser clients send no Origin and are allowed.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()

	userID := ""
	if u, ok := middleware.UserFromContext(r.Context()); ok {
		userID = u.ID
	}
	id, send := h.Hub.Register(userID)
	defer h.Hub.Unsubscribe(id) // runs before CloseNow (LIFO): stops writer first
	h.Hub.Subscribe(id, "ping")

	// Cancelled when the writer hits a dead connection, so the reader's blocked
	// c.Read returns and the handler unwinds (Unsubscribe + CloseNow) instead of
	// leaking the registration until an unrelated read error.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Tell the client its hub id so it can request sensitive topic grants (logs)
	// via the authorized POST /api/logs/subscribe endpoint.
	if idMsg, err := json.Marshal(map[string]string{"client_id": string(id)}); err == nil {
		_ = c.Write(ctx, websocket.MessageText, idMsg)
	}

	go func() {
		for msg := range send {
			if err := c.Write(ctx, websocket.MessageText, msg); err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		// Liveness probe (kept for the SP0 smoke test).
		if string(data) == "ping" {
			h.Hub.Broadcast("ping", []byte("pong"))
			continue
		}
		// Control: subscribe/unsubscribe to topics (e.g. "tree:<nodeId>").
		var cmd wsCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			continue
		}
		if cmd.Subscribe != "" && clientMaySubscribe(cmd.Subscribe) {
			h.Hub.Subscribe(id, cmd.Subscribe)
		}
		// (Per-topic unsubscribe is a hub enhancement; full Unsubscribe happens
		// on disconnect. cmd.Unsubscribe is accepted but a no-op for now.)
		_ = cmd.Unsubscribe
	}
}

package api

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
)

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

	id, send := h.Hub.Register()
	defer h.Hub.Unsubscribe(id) // runs before CloseNow (LIFO): stops writer first
	h.Hub.Subscribe(id, "ping")

	// Cancelled when the writer hits a dead connection, so the reader's blocked
	// c.Read returns and the handler unwinds (Unsubscribe + CloseNow) instead of
	// leaking the registration until an unrelated read error.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

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
		if string(data) == "ping" {
			h.Hub.Broadcast("ping", []byte("pong"))
		} else {
			h.Hub.Broadcast("ping", data)
		}
	}
}

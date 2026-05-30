package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/ssh"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/middleware"
)

// terminalControl is the client->server control message (sent as a TEXT frame).
// Raw keystroke input is sent as BINARY frames; PTY output flows back as BINARY.
type terminalControl struct {
	Type string `json:"type"` // "resize"
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// NodeTerminal opens an interactive shell on a node over SSH and bridges it to a
// WebSocket PTY. Admin-only (arbitrary host shell access) and audited. The SSH
// dial happens before the WS upgrade so failures surface as HTTP errors.
//
// Wire protocol once connected:
//   - client -> server: BINARY = stdin bytes; TEXT = JSON terminalControl (resize)
//   - server -> client: BINARY = PTY output
func (h *Handlers) NodeTerminal(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")

	client, err := h.Files.DialSSH(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_dial_failed")
		return
	}
	defer client.Close()

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{middleware.WSBearerSubprotocol},
	})
	if err != nil {
		return
	}
	defer c.CloseNow()

	// Audit the shell open (a mutating, security-relevant action).
	if uid, ok := middleware.UserFromContext(r.Context()); ok {
		_ = h.Activity.Append(r.Context(), activity.Entry{
			UserID: &uid.ID, Action: "node.terminal", TargetType: ptr("node"), TargetID: &nodeID,
			Result: activity.ResultSuccess,
		})
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	session, err := client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		return
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return
	}

	if err := session.Shell(); err != nil {
		return
	}

	// Pump PTY output -> WS (binary). Cancels the context on the first write/read
	// error so the reader loop below unwinds and the handler tears down.
	go func() {
		buf := make([]byte, 8192)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				if werr := c.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					cancel()
					return
				}
			}
			if rerr != nil {
				cancel()
				return
			}
		}
	}()

	// Read WS -> PTY stdin. Text frames are resize control; binary frames are input.
	for {
		typ, data, rerr := c.Read(ctx)
		if rerr != nil {
			break
		}
		if typ == websocket.MessageText {
			var ctl terminalControl
			if json.Unmarshal(data, &ctl) == nil && ctl.Type == "resize" && ctl.Cols > 0 && ctl.Rows > 0 {
				_ = session.WindowChange(ctl.Rows, ctl.Cols)
			}
			continue
		}
		if _, werr := stdin.Write(data); werr != nil {
			break
		}
	}

	// Best-effort: hang up the remote shell when the socket closes.
	_ = session.Signal(ssh.SIGHUP)
}

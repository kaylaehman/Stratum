// Package nodeconn caches per-node transport clients (Docker, Proxmox) built
// from a node's sealed credentials, so the inventory poller doesn't rebuild
// them every cycle. Clients here are cheap handles (Docker SDK client, stateless
// Proxmox HTTP client), so a simple cache + Invalidate suffices — no async
// health-repair is needed because the poll path holds no persistent SSH dial.
package nodeconn

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"syscall"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/proxmox"
)

// Clients are the transport clients for one node. Either may be nil when the
// node lacks that capability.
type Clients struct {
	Node    db.Node
	Docker  *docker.Client
	Proxmox *proxmox.Client
}

// Manager builds and caches per-node Clients.
type Manager struct {
	store  db.Store
	cipher *crypto.Cipher

	mu    sync.Mutex
	cache map[string]*Clients
}

// NewManager creates a Manager.
func NewManager(store db.Store, cipher *crypto.Cipher) *Manager {
	return &Manager{store: store, cipher: cipher, cache: map[string]*Clients{}}
}

// Get returns the cached clients for a node, building them on first use from
// the node's decrypted credentials and capabilities.
func (m *Manager) Get(ctx context.Context, nodeID string) (*Clients, error) {
	m.mu.Lock()
	if c, ok := m.cache[nodeID]; ok {
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()

	node, err := m.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	creds, err := nodes.OpenCredentials(m.cipher, node.CredentialsEncrypted)
	if err != nil {
		return nil, err
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))

	cl := &Clients{Node: node}
	// Build a Docker client only when the node carries an explicit Docker
	// endpoint. docker.New("") falls back to the LOCAL Docker socket (the
	// Stratum backend's own daemon) — if we did that for a remote node whose
	// Docker is only reachable over SSH (Docker detected via the SSH probe, no
	// TCP/TLS endpoint stored), the poller would enumerate Stratum's own
	// containers and attribute them to the remote node. Refuse instead: the node
	// still reports reachable via the SSH fallback, just with no containers,
	// which is correct until Docker-over-SSH transport is wired up.
	if caps.Docker && node.DockerEndpoint != "" {
		var tls *docker.TLS
		if creds.DockerTLSCA != "" || creds.DockerTLSCert != "" || creds.DockerTLSKey != "" {
			tls = &docker.TLS{CA: creds.DockerTLSCA, Cert: creds.DockerTLSCert, Key: creds.DockerTLSKey}
		}
		if dc, derr := docker.New(node.DockerEndpoint, tls); derr == nil {
			cl.Docker = dc
		} else {
			slog.Warn("nodeconn: docker client build failed", "node", nodeID, "error", derr)
		}
	} else if caps.Docker {
		slog.Info("nodeconn: node has Docker capability but no Docker endpoint; containers will not be enumerated (Docker-over-SSH transport not yet supported)",
			"node", nodeID)
	}
	if caps.Proxmox && node.ProxmoxEndpoint != "" && creds.ProxmoxTokenID != "" {
		cl.Proxmox = proxmox.New(node.ProxmoxEndpoint, creds.ProxmoxTokenID, creds.ProxmoxSecret, node.ProxmoxTLSInsecure)
	}

	// Double-checked insert: if another caller built the clients while we were
	// building (no lock held during build), discard ours and use theirs.
	m.mu.Lock()
	if existing, ok := m.cache[nodeID]; ok {
		m.mu.Unlock()
		if cl.Docker != nil {
			_ = cl.Docker.Close() // safe: no one else holds this discarded build
		}
		return existing, nil
	}
	m.cache[nodeID] = cl
	m.mu.Unlock()
	return cl, nil
}

// Invalidate drops a node's cached clients (call on node update/delete). It does
// NOT close the Docker client: a concurrent in-flight poll may still hold the
// pointer, and closing it would risk use-after-close. The Docker SDK client only
// wraps an http.Client (idle keep-alive connections), which the GC reclaims.
func (m *Manager) Invalidate(nodeID string) {
	m.mu.Lock()
	delete(m.cache, nodeID)
	m.mu.Unlock()
}

// Rebuild forces a fresh client build for nodeID (dropping the cache entry) and
// returns the new Clients. It is safe to call concurrently: it delegates to Get
// after invalidating. Use this when a Docker call returns a transport-level
// error (EOF, connection reset) indicating the cached connection is stale.
func (m *Manager) Rebuild(ctx context.Context, nodeID string) (*Clients, error) {
	m.Invalidate(nodeID)
	return m.Get(ctx, nodeID)
}

// IsTransportError reports whether err indicates a dead or reset TCP/TLS
// connection rather than a Docker-protocol or application error. These errors
// mean the cached http.Transport has a stale keep-alive connection and the call
// should be retried on a fresh client, NOT treated as "node unreachable".
func IsTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	// syscall.ECONNRESET / EPIPE — the peer reset or closed the connection.
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	// Catch-all string match for wrapped transport errors not reachable via
	// errors.Is on Windows or via the Docker SDK's error wrapping.
	msg := err.Error()
	for _, fragment := range []string{
		"connection reset by peer",
		"broken pipe",
		"EOF",
		"use of closed network connection",
		"connection refused",
		"forcibly closed",
	} {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}

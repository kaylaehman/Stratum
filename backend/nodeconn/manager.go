// Package nodeconn caches per-node transport clients (Docker, Proxmox) built
// from a node's sealed credentials, so the inventory poller doesn't rebuild
// them every cycle. Clients here are cheap handles (Docker SDK client, stateless
// Proxmox HTTP client), so a simple cache + Invalidate suffices — no async
// health-repair is needed because the poll path holds no persistent SSH dial.
package nodeconn

import (
	"context"
	"sync"

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
	if caps.Docker {
		var tls *docker.TLS
		if creds.DockerTLSCA != "" || creds.DockerTLSCert != "" || creds.DockerTLSKey != "" {
			tls = &docker.TLS{CA: creds.DockerTLSCA, Cert: creds.DockerTLSCert, Key: creds.DockerTLSKey}
		}
		if dc, derr := docker.New(node.DockerEndpoint, tls); derr == nil {
			cl.Docker = dc
		}
	}
	if caps.Proxmox && node.ProxmoxEndpoint != "" && creds.ProxmoxTokenID != "" {
		cl.Proxmox = proxmox.New(node.ProxmoxEndpoint, creds.ProxmoxTokenID, creds.ProxmoxSecret, node.ProxmoxTLSInsecure)
	}

	m.mu.Lock()
	m.cache[nodeID] = cl
	m.mu.Unlock()
	return cl, nil
}

// Invalidate drops a node's cached clients (call on node update/delete),
// closing the Docker client if present.
func (m *Manager) Invalidate(nodeID string) {
	m.mu.Lock()
	c, ok := m.cache[nodeID]
	delete(m.cache, nodeID)
	m.mu.Unlock()
	if ok && c.Docker != nil {
		_ = c.Docker.Close()
	}
}

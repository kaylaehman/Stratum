// Package topology builds the Docker network topology (Feature 29): per node, the
// networks with their container memberships, plus the node's containers so the UI
// can flag isolated (no network) and host-network containers. Live read-only data
// from the daemon — nothing is persisted.
package topology

import (
	"context"
	"sort"
	"strings"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/nodeconn"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// networkLister is the slice of the Docker client the topology service depends
// on. *docker.Client satisfies it (its ListNetworks has this exact signature),
// so production wiring is unchanged; tests inject a fake to prove networks are
// returned (and that transport errors propagate for the API-layer retry)
// without a live daemon.
type networkLister interface {
	ListNetworks(ctx context.Context) ([]docker.NetworkInfo, error)
}

// ContainerNode is one container in the topology, annotated with the networks it
// belongs to and whether it is isolated or on the host network.
type ContainerNode struct {
	DockerID    string   `json:"docker_id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Networks    []string `json:"networks"`     // network names this container is attached to
	Isolated    bool     `json:"isolated"`     // attached to no network
	HostNetwork bool     `json:"host_network"` // attached to the "host" network
}

// Topology is one node's network graph data.
type Topology struct {
	NodeID      string               `json:"node_id"`
	NodeStatus  string               `json:"node_status"`  // poller-authoritative: "ok" | "unreachable" | "error" | "unknown"
	DockerError string               `json:"docker_error"` // non-empty when the Docker daemon could not be reached
	Networks    []docker.NetworkInfo `json:"networks"`
	Containers  []ContainerNode      `json:"containers"`
}

// Service builds topology for docker nodes.
type Service struct {
	store    db.Store
	provider ClientProvider

	// listerFor resolves the network lister for a node. It defaults to the
	// provider (returning the concrete *docker.Client, which satisfies
	// networkLister). Only tests override it, to inject a working/failing fake
	// without a live daemon. Keeping it a seam — rather than changing the public
	// ClientProvider signature — leaves main.go wiring untouched.
	listerFor func(ctx context.Context, nodeID string) (networkLister, error)
}

// New builds the service.
func New(store db.Store, provider ClientProvider) *Service {
	s := &Service{store: store, provider: provider}
	s.listerFor = func(ctx context.Context, nodeID string) (networkLister, error) {
		c, err := s.provider(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
	return s
}

// ForNode returns the network topology for one node: its networks (with
// endpoints) and its containers annotated with membership/isolation.
// The node's authoritative reachability status is always read from the DB
// (maintained by the poller) — never re-derived from the Docker dial here.
// When the Docker daemon is unreachable but the node is otherwise reachable
// (e.g. SSH-only or Docker not yet configured), the response carries an empty
// topology with a descriptive docker_error instead of propagating an error.
func (s *Service) ForNode(ctx context.Context, nodeID string) (Topology, error) {
	node, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return Topology{}, err
	}
	nodeStatus := node.Status
	if nodeStatus == "" {
		nodeStatus = "unknown"
	}

	base := Topology{
		NodeID:     nodeID,
		NodeStatus: nodeStatus,
		Networks:   []docker.NetworkInfo{},
		Containers: []ContainerNode{},
	}

	client, err := s.listerFor(ctx, nodeID)
	if err != nil {
		// No usable Docker client for this node (e.g. Docker capability inferred
		// over SSH with no TCP/TLS endpoint configured). The node may still be
		// fully reachable per the poller — degrade to an empty topology with a
		// descriptive marker rather than failing the request.
		base.DockerError = "docker_client_unavailable"
		return base, nil
	}

	networks, err := client.ListNetworks(ctx)
	if err != nil {
		// A transport-level failure (stale cached keep-alive connection after a
		// daemon restart / idle reset) MUST be propagated so the API layer can
		// rebuild the cached client and retry. Swallowing it into DockerError
		// here is exactly why networks came back empty while the poller — which
		// refreshes on its own cycle — still showed containers. Only a genuine
		// application/protocol error degrades to an empty topology.
		if nodeconn.IsTransportError(err) {
			return Topology{}, err
		}
		base.DockerError = "docker_list_networks_failed"
		return base, nil
	}

	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return Topology{}, err
	}

	base.Containers = buildContainerNodes(networks, containers)
	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	base.Networks = networks
	return base, nil
}

// buildContainerNodes annotates each container with the networks it belongs to
// (matched by docker id against network endpoints) and the isolated/host-network
// flags. Pure function — unit-tested without a docker client. Matching is
// prefix-tolerant (full 64-char vs short 12-char id) so a short id stored
// anywhere can't silently mark a container isolated.
func buildContainerNodes(networks []docker.NetworkInfo, containers []db.Container) []ContainerNode {
	cnodes := make([]ContainerNode, 0, len(containers))
	for _, c := range containers {
		var nets []string
		for _, n := range networks {
			for _, ep := range n.Endpoints {
				if idMatch(ep.ContainerID, c.DockerID) {
					nets = append(nets, n.Name)
					break
				}
			}
		}
		sort.Strings(nets)
		cnodes = append(cnodes, ContainerNode{
			DockerID:    c.DockerID,
			Name:        c.Name,
			Status:      c.Status,
			Networks:    nets,
			Isolated:    len(nets) == 0,
			HostNetwork: contains(nets, "host"),
		})
	}
	sort.Slice(cnodes, func(i, j int) bool { return cnodes[i].Name < cnodes[j].Name })
	return cnodes
}

// idMatch reports whether two docker container ids refer to the same container,
// tolerating a short (≥12-char) id being a prefix of the full 64-char id.
func idMatch(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) >= 12 && strings.HasPrefix(b, a) {
		return true
	}
	if len(b) >= 12 && strings.HasPrefix(a, b) {
		return true
	}
	return false
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

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
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

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
}

// New builds the service.
func New(store db.Store, provider ClientProvider) *Service {
	return &Service{store: store, provider: provider}
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

	client, err := s.provider(ctx, nodeID)
	if err != nil {
		base.DockerError = "docker_client_unavailable"
		return base, nil
	}

	networks, err := client.ListNetworks(ctx)
	if err != nil {
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

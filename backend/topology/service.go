// Package topology builds the Docker network topology (Feature 29): per node, the
// networks with their container memberships, plus the node's containers so the UI
// can flag isolated (no network) and host-network containers. Live read-only data
// from the daemon — nothing is persisted.
package topology

import (
	"context"
	"sort"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// ContainerNode is one container in the topology, annotated with the networks it
// belongs to and whether it is isolated or on the host network.
type ContainerNode struct {
	DockerID   string   `json:"docker_id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Networks   []string `json:"networks"`     // network names this container is attached to
	Isolated   bool     `json:"isolated"`     // attached to no network
	HostNetwork bool    `json:"host_network"` // attached to the "host" network
}

// Topology is one node's network graph data.
type Topology struct {
	NodeID     string               `json:"node_id"`
	Networks   []docker.NetworkInfo `json:"networks"`
	Containers []ContainerNode      `json:"containers"`
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
func (s *Service) ForNode(ctx context.Context, nodeID string) (Topology, error) {
	client, err := s.provider(ctx, nodeID)
	if err != nil {
		return Topology{}, err
	}
	networks, err := client.ListNetworks(ctx)
	if err != nil {
		return Topology{}, err
	}
	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return Topology{}, err
	}

	cnodes := buildContainerNodes(networks, containers)
	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	return Topology{NodeID: nodeID, Networks: networks, Containers: cnodes}, nil
}

// buildContainerNodes annotates each container with the networks it belongs to
// (matched by docker id against network endpoints) and the isolated/host-network
// flags. Pure function — unit-tested without a docker client.
func buildContainerNodes(networks []docker.NetworkInfo, containers []db.Container) []ContainerNode {
	netsByContainer := map[string][]string{}
	for _, n := range networks {
		for _, ep := range n.Endpoints {
			netsByContainer[ep.ContainerID] = append(netsByContainer[ep.ContainerID], n.Name)
		}
	}
	cnodes := make([]ContainerNode, 0, len(containers))
	for _, c := range containers {
		nets := netsByContainer[c.DockerID]
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

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Package depgraph builds the container dependency graph (Feature 16): a single
// graph of a node's containers, networks, and volumes, with edges for
// container→network membership and container→volume attachment. Live read-only
// data assembled from the daemon (networks) + the SP7 mount index (volumes) +
// the inventory store (containers). Nothing is persisted.
package depgraph

import (
	"context"
	"sort"
	"strings"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/mountindex"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Node kinds.
const (
	KindContainer = "container"
	KindNetwork   = "network"
	KindVolume    = "volume"
)

// GraphNode is one vertex. ID is namespaced ("container:<id>", "network:<name>",
// "volume:<name>") so kinds never collide. Status/ComposeProject are populated
// for container nodes only (the latter drives the compose-project filter).
type GraphNode struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Label          string `json:"label"`
	Status         string `json:"status,omitempty"`
	ComposeProject string `json:"compose_project,omitempty"`
	Driver         string `json:"driver,omitempty"` // network driver
}

// GraphEdge connects a container node to a network or volume node. Kind is
// "network" or "volume".
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// Graph is the assembled dependency graph for one node.
type Graph struct {
	NodeID string      `json:"node_id"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
}

// Service assembles dependency graphs for docker nodes.
type Service struct {
	store    db.Store
	provider ClientProvider
	mounts   *mountindex.Index
}

// New builds the service.
func New(store db.Store, provider ClientProvider, mounts *mountindex.Index) *Service {
	return &Service{store: store, provider: provider, mounts: mounts}
}

func containerNodeID(id string) string { return KindContainer + ":" + id }
func networkNodeID(name string) string { return KindNetwork + ":" + name }
func volumeNodeID(name string) string  { return KindVolume + ":" + name }

// ForNode assembles the graph: container nodes from inventory, network nodes +
// membership edges from the daemon, volume nodes + attachment edges from the
// mount index.
func (s *Service) ForNode(ctx context.Context, nodeID string) (Graph, error) {
	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return Graph{}, err
	}
	client, err := s.provider(ctx, nodeID)
	if err != nil {
		return Graph{}, err
	}
	networks, err := client.ListNetworks(ctx)
	if err != nil {
		return Graph{}, err
	}

	var nodes []GraphNode
	var edges []GraphEdge

	// Container nodes + a docker-id -> internal-id index for matching network
	// endpoints (which are keyed by docker id).
	internalByDocker := map[string]string{}
	for _, c := range containers {
		internalByDocker[c.DockerID] = c.ID
		nodes = append(nodes, GraphNode{
			ID:             containerNodeID(c.ID),
			Kind:           KindContainer,
			Label:          c.Name,
			Status:         c.Status,
			ComposeProject: c.ComposeProject,
		})
	}

	// Network nodes + container→network edges.
	for _, n := range networks {
		nodes = append(nodes, GraphNode{
			ID:     networkNodeID(n.Name),
			Kind:   KindNetwork,
			Label:  n.Name,
			Driver: n.Driver,
		})
		for _, ep := range n.Endpoints {
			if internal := matchInternal(internalByDocker, ep.ContainerID); internal != "" {
				edges = append(edges, GraphEdge{
					Source: containerNodeID(internal),
					Target: networkNodeID(n.Name),
					Kind:   KindNetwork,
				})
			}
		}
	}

	// Volume nodes + container→volume edges (from the mount index).
	edges = append(edges, s.volumeEdges(ctx, nodeID, &nodes)...)

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		return nodes[i].Label < nodes[j].Label
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
	return Graph{NodeID: nodeID, Nodes: nodes, Edges: edges}, nil
}

// volumeEdges adds a volume node per distinct named volume mounted on the node
// and a container→volume edge per mount, using the freshened mount index. Mount
// rows are keyed by the internal container id. Best-effort: a seed failure adds
// no volume edges (the rest of the graph still renders).
func (s *Service) volumeEdges(ctx context.Context, nodeID string, nodes *[]GraphNode) []GraphEdge {
	if err := s.mounts.EnsureFresh(ctx, nodeID); err != nil {
		return nil
	}
	rows, err := s.store.ListMountsByNode(ctx, nodeID)
	if err != nil {
		return nil
	}
	seenVol := map[string]bool{}
	var edges []GraphEdge
	for _, m := range rows {
		if m.VolumeName == "" {
			continue
		}
		if !seenVol[m.VolumeName] {
			seenVol[m.VolumeName] = true
			*nodes = append(*nodes, GraphNode{
				ID:    volumeNodeID(m.VolumeName),
				Kind:  KindVolume,
				Label: m.VolumeName,
			})
		}
		edges = append(edges, GraphEdge{
			Source: containerNodeID(m.ContainerID),
			Target: volumeNodeID(m.VolumeName),
			Kind:   KindVolume,
		})
	}
	return edges
}

// matchInternal resolves a network endpoint's docker id to an internal container
// id, tolerating full-vs-short id forms.
func matchInternal(internalByDocker map[string]string, dockerID string) string {
	if id, ok := internalByDocker[dockerID]; ok {
		return id
	}
	for dock, internal := range internalByDocker {
		if len(dockerID) >= 12 && strings.HasPrefix(dock, dockerID) {
			return internal
		}
		if len(dock) >= 12 && strings.HasPrefix(dockerID, dock) {
			return internal
		}
	}
	return ""
}

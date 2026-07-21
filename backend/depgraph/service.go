// Package depgraph builds the container dependency graph (Feature 16): a single
// graph of a node's containers, networks, and volumes, with edges for
// container→network membership and container→volume attachment. Live read-only
// data assembled from the daemon (networks) + the SP7 mount index (volumes) +
// the inventory store (containers). Nothing is persisted.
package depgraph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/mountindex"
	"github.com/KAE-Labs/stratum/backend/nodeconn"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// ErrNoDockerClient signals that no Docker client could be built for the node —
// the node has the Docker capability but no usable transport (e.g. Docker
// inferred over SSH with no TCP/TLS endpoint). It is NOT a reachability failure;
// the API layer maps it to "docker_not_available" rather than "node_unreachable".
var ErrNoDockerClient = errors.New("depgraph: no docker client for node")

// errNoClient wraps the underlying provider error while marking it as
// ErrNoDockerClient (so callers can errors.Is it) and preserving the cause for
// logs.
func errNoClient(cause error) error {
	if cause == nil {
		return ErrNoDockerClient
	}
	return fmt.Errorf("%w: %v", ErrNoDockerClient, cause)
}

// networkLister is the slice of the Docker client the depgraph service depends
// on. *docker.Client satisfies it, so production wiring is unchanged; tests
// inject a fake to assemble graphs (and to distinguish a genuinely unreachable
// node from one whose Docker is merely unconfigured) without a live daemon.
type networkLister interface {
	ListNetworks(ctx context.Context) ([]docker.NetworkInfo, error)
}

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

	// listerFor resolves the network lister for a node. Defaults to the provider
	// (*docker.Client satisfies networkLister); only tests override it. Keeping a
	// seam rather than changing ClientProvider leaves main.go wiring untouched.
	listerFor func(ctx context.Context, nodeID string) (networkLister, error)
}

// New builds the service.
func New(store db.Store, provider ClientProvider, mounts *mountindex.Index) *Service {
	s := &Service{store: store, provider: provider, mounts: mounts}
	s.listerFor = func(ctx context.Context, nodeID string) (networkLister, error) {
		c, err := s.provider(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
	return s
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
	client, err := s.listerFor(ctx, nodeID)
	if err != nil {
		// No usable Docker client (e.g. Docker capability inferred over SSH with
		// no TCP/TLS endpoint configured). This is NOT "node unreachable": the
		// node may be fully up per the poller. Return ErrNoDockerClient so the
		// API layer can answer accurately ("docker_not_available") instead of a
		// misleading 502 node_unreachable. A genuine transport failure surfaces
		// later from ListNetworks (and is retried), not here.
		return Graph{}, errNoClient(err)
	}
	// Network listing: a TRANSPORT error (stale cached keep-alive after a daemon
	// restart / idle reset) is propagated so the API can rebuild the client and
	// retry — swallowing it is exactly why the dependency graph showed no
	// networks while the poller still listed containers. A genuine application
	// error degrades best-effort to a graph with no network nodes/edges
	// (containers + volumes still render), matching the volume-edge path.
	networks, nerr := client.ListNetworks(ctx)
	if nerr != nil {
		if nodeconn.IsTransportError(nerr) {
			return Graph{}, nerr
		}
		networks = nil
	}

	var nodes []GraphNode
	var edges []GraphEdge

	// Container nodes + indexes: docker-id -> internal-id (to match network
	// endpoints, keyed by docker id) and a valid-internal-id set (to drop edges
	// from stale mount rows whose container no longer exists).
	internalByDocker := map[string]string{}
	validContainer := map[string]bool{}
	for _, c := range containers {
		internalByDocker[c.DockerID] = c.ID
		validContainer[c.ID] = true
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
	edges = append(edges, s.volumeEdges(ctx, nodeID, validContainer, &nodes)...)

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
func (s *Service) volumeEdges(ctx context.Context, nodeID string, validContainer map[string]bool, nodes *[]GraphNode) []GraphEdge {
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
		// Skip a stale row whose container is no longer present, else the edge
		// would point at a container node that isn't in the graph.
		if m.VolumeName == "" || !validContainer[m.ContainerID] {
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
// id, tolerating full-vs-short id forms. Assumes ids don't collide on a 12-char
// prefix (true for real docker ids); on the astronomically-unlikely ambiguous
// prefix it returns the first match in map order.
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

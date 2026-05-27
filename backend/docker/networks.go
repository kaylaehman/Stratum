package docker

import (
	"context"
	"sort"

	"github.com/docker/docker/api/types/network"
)

// NetworkEndpoint is one container's attachment to a network.
type NetworkEndpoint struct {
	ContainerID string `json:"container_id"` // docker container id
	Name        string `json:"name"`         // container name as reported by the network
	IPv4Address string `json:"ipv4_address"`
}

// NetworkInfo is one Docker network with its members.
type NetworkInfo struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Driver    string            `json:"driver"`
	Scope     string            `json:"scope"`
	Internal  bool              `json:"internal"`
	Subnet    string            `json:"subnet"`
	Gateway   string            `json:"gateway"`
	Endpoints []NetworkEndpoint `json:"endpoints"`
}

// ListNetworks enumerates networks and inspects each for container membership
// (the list endpoint does not populate Containers). Subnet/gateway come from the
// first IPAM config entry.
func (c *Client) ListNetworks(ctx context.Context) ([]NetworkInfo, error) {
	summaries, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NetworkInfo, 0, len(summaries))
	for _, s := range summaries {
		ins, err := c.cli.NetworkInspect(ctx, s.ID, network.InspectOptions{})
		if err != nil {
			// Network removed mid-list: fall back to the summary (no members).
			// network.Summary is a type alias of network.Inspect in this SDK, so
			// this is a valid value (just without the Containers map populated).
			out = append(out, mapNetwork(s))
			continue
		}
		out = append(out, mapNetwork(ins))
	}
	return out, nil
}

func mapNetwork(n network.Inspect) NetworkInfo {
	subnet, gateway := "", ""
	if len(n.IPAM.Config) > 0 {
		subnet = n.IPAM.Config[0].Subnet
		gateway = n.IPAM.Config[0].Gateway
	}
	endpoints := make([]NetworkEndpoint, 0, len(n.Containers))
	for cid, ep := range n.Containers {
		endpoints = append(endpoints, NetworkEndpoint{
			ContainerID: cid,
			Name:        ep.Name,
			IPv4Address: ep.IPv4Address,
		})
	}
	// Map iteration is unordered; sort for stable responses (avoids spurious
	// UI re-renders on polling).
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].ContainerID < endpoints[j].ContainerID })
	return NetworkInfo{
		ID:        n.ID,
		Name:      n.Name,
		Driver:    n.Driver,
		Scope:     n.Scope,
		Internal:  n.Internal,
		Subnet:    subnet,
		Gateway:   gateway,
		Endpoints: endpoints,
	}
}

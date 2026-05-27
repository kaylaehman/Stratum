package mountindex

import (
	"context"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// MountView is a container's mount, annotated with the shared flag.
type MountView struct {
	ContainerID string `json:"container_id"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	VolumeName  string `json:"volume_name,omitempty"`
	RW          bool   `json:"rw"`
	Shared      bool   `json:"shared"`    // mounted into >1 container on this node
	Traceable   bool   `json:"traceable"` // bind or volume (tmpfs etc. are not)
}

// ReverseHit is a container that mounts a path related to the queried host path.
type ReverseHit struct {
	ContainerID string               `json:"container_id"`
	Source      string               `json:"source"`
	Destination string               `json:"destination"`
	RW          bool                 `json:"rw"`
	Relation    docker.Relationship  `json:"relation"`
}

// SharedEntry is a source (bind path or volume name) mounted into >1 container.
type SharedEntry struct {
	Kind         string   `json:"kind"` // bind | volume
	Key          string   `json:"key"`  // normalized source path (bind) or volume name (volume)
	ContainerIDs []string `json:"container_ids"`
}

func traceable(t string) bool { return t == "bind" || t == "volume" }

// sharedKeys computes which bind sources and volume names are mounted into more
// than one distinct container on the node.
func sharedKeys(rows []db.MountRow) (binds, volumes map[string]bool) {
	bindSet := map[string]map[string]bool{}
	volSet := map[string]map[string]bool{}
	for _, m := range rows {
		switch m.Type {
		case "bind":
			add(bindSet, m.NormalizedSource, m.ContainerID)
		case "volume":
			if m.VolumeName != "" {
				add(volSet, m.VolumeName, m.ContainerID)
			}
		}
	}
	return moreThanOne(bindSet), moreThanOne(volSet)
}

func add(m map[string]map[string]bool, key, ctr string) {
	if m[key] == nil {
		m[key] = map[string]bool{}
	}
	m[key][ctr] = true
}
func moreThanOne(m map[string]map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, set := range m {
		if len(set) > 1 {
			out[k] = true
		}
	}
	return out
}

// Forward returns a container's mounts with the shared flag set.
func (ix *Index) Forward(ctx context.Context, nodeID, containerID string) ([]MountView, error) {
	if err := ix.ensureFresh(ctx, nodeID); err != nil {
		return nil, err
	}
	nodeRows, err := ix.store.ListMountsByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	sharedBinds, sharedVols := sharedKeys(nodeRows)

	rows, err := ix.store.ListMountsByContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	out := make([]MountView, 0, len(rows))
	for _, m := range rows {
		shared := (m.Type == "bind" && sharedBinds[m.NormalizedSource]) ||
			(m.Type == "volume" && m.VolumeName != "" && sharedVols[m.VolumeName])
		out = append(out, MountView{
			ContainerID: m.ContainerID, Type: m.Type, Source: m.Source, Destination: m.Destination,
			VolumeName: m.VolumeName, RW: m.RW, Shared: shared, Traceable: traceable(m.Type),
		})
	}
	return out, nil
}

// Reverse returns containers whose bind source equals, is a parent of, or is a
// child of hostPath. Volume mounts are not host-path-addressable (skipped).
func (ix *Index) Reverse(ctx context.Context, nodeID, hostPath string) ([]ReverseHit, error) {
	if err := ix.ensureFresh(ctx, nodeID); err != nil {
		return nil, err
	}
	rows, err := ix.store.ListMountsByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	var hits []ReverseHit
	for _, m := range rows {
		if m.Type != "bind" {
			continue
		}
		rel := docker.Relation(m.NormalizedSource, hostPath)
		if rel == docker.RelUnrelated {
			continue
		}
		hits = append(hits, ReverseHit{
			ContainerID: m.ContainerID, Source: m.Source, Destination: m.Destination, RW: m.RW, Relation: rel,
		})
	}
	return hits, nil
}

// Shared returns the bind sources and volume names mounted into >1 container.
func (ix *Index) Shared(ctx context.Context, nodeID string) ([]SharedEntry, error) {
	if err := ix.ensureFresh(ctx, nodeID); err != nil {
		return nil, err
	}
	rows, err := ix.store.ListMountsByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	bindMembers := map[string]map[string]bool{}
	volMembers := map[string]map[string]bool{}
	for _, m := range rows {
		switch m.Type {
		case "bind":
			add(bindMembers, m.NormalizedSource, m.ContainerID)
		case "volume":
			if m.VolumeName != "" {
				add(volMembers, m.VolumeName, m.ContainerID)
			}
		}
	}
	var out []SharedEntry
	out = appendShared(out, "bind", bindMembers)
	out = appendShared(out, "volume", volMembers)
	return out, nil
}

func appendShared(out []SharedEntry, kind string, members map[string]map[string]bool) []SharedEntry {
	for key, set := range members {
		if len(set) <= 1 {
			continue
		}
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		out = append(out, SharedEntry{Kind: kind, Key: key, ContainerIDs: ids})
	}
	return out
}

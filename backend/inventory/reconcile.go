package inventory

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/db"
)

func vmKey(v db.VM) string  { return strconv.Itoa(v.ProxmoxVMID) + "/" + v.Kind }
func ctrKey(c db.Container) string { return c.DockerID }

func ptrVMView(v db.VM) *VMView           { vv := FromVM(v); return &vv }
func ptrCtrView(c db.Container) *ContainerView { cv := FromContainer(c); return &cv }

// reconcileVMs diffs enumerated guests against the DB for a node, applies the
// 2-poll gone grace, and returns the resulting deltas (without seq — the caller
// stamps it).
func reconcileVMs(ctx context.Context, store db.Store, nodeID string, enumerated []db.VM) ([]Delta, error) {
	existing, err := store.ListVMsByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]db.VM, len(existing))
	for _, e := range existing {
		byKey[vmKey(e)] = e
	}
	now := time.Now()
	seen := make(map[string]bool, len(enumerated))
	var deltas []Delta

	for _, en := range enumerated {
		k := vmKey(en)
		seen[k] = true
		en.LastSeen = now
		if cur, ok := byKey[k]; ok {
			en.ID = cur.ID
			changed := cur.Status != en.Status || cur.Name != en.Name ||
				cur.ProxmoxNode != en.ProxmoxNode || cur.Stale || cur.GoneSince != nil
			en.Stale = false
			en.GoneSince = nil
			if err := store.UpsertVM(ctx, en); err != nil {
				return nil, err
			}
			if changed {
				deltas = append(deltas, Delta{Op: OpUpdated, Kind: KindVM, NodeID: nodeID, VM: ptrVMView(en)})
			}
		} else {
			en.ID = uuid.NewString()
			if err := store.UpsertVM(ctx, en); err != nil {
				return nil, err
			}
			deltas = append(deltas, Delta{Op: OpAdded, Kind: KindVM, NodeID: nodeID, VM: ptrVMView(en)})
		}
	}

	for k, cur := range byKey {
		if seen[k] {
			continue
		}
		if cur.GoneSince == nil {
			// First absence: mark stale, start the grace window.
			cur.GoneSince = &now
			cur.Stale = true
			if err := store.UpsertVM(ctx, cur); err != nil {
				return nil, err
			}
			deltas = append(deltas, Delta{Op: OpUpdated, Kind: KindVM, NodeID: nodeID, VM: ptrVMView(cur)})
		} else {
			// Absent for a second consecutive poll: remove.
			if err := store.DeleteVM(ctx, cur.ID); err != nil {
				return nil, err
			}
			deltas = append(deltas, Delta{Op: OpRemoved, Kind: KindVM, NodeID: nodeID, VM: ptrVMView(cur)})
		}
	}
	return deltas, nil
}

// reconcileContainers is the container equivalent of reconcileVMs. A recreated
// container has a new docker_id, so the old key is absent (removed after grace)
// and the new key is added.
func reconcileContainers(ctx context.Context, store db.Store, nodeID string, enumerated []db.Container) ([]Delta, error) {
	existing, err := store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]db.Container, len(existing))
	for _, e := range existing {
		byKey[ctrKey(e)] = e
	}
	now := time.Now()
	seen := make(map[string]bool, len(enumerated))
	var deltas []Delta

	for _, en := range enumerated {
		k := ctrKey(en)
		seen[k] = true
		en.LastSeen = now
		if cur, ok := byKey[k]; ok {
			en.ID = cur.ID
			changed := cur.Status != en.Status || cur.Name != en.Name || cur.Image != en.Image ||
				cur.ImageID != en.ImageID || cur.ComposeProject != en.ComposeProject ||
				cur.Stale || cur.GoneSince != nil
			en.Stale = false
			en.GoneSince = nil
			if err := store.UpsertContainer(ctx, en); err != nil {
				return nil, err
			}
			if changed {
				deltas = append(deltas, Delta{Op: OpUpdated, Kind: KindContainer, NodeID: nodeID, Container: ptrCtrView(en)})
			}
		} else {
			en.ID = uuid.NewString()
			if err := store.UpsertContainer(ctx, en); err != nil {
				return nil, err
			}
			deltas = append(deltas, Delta{Op: OpAdded, Kind: KindContainer, NodeID: nodeID, Container: ptrCtrView(en)})
		}
	}

	for k, cur := range byKey {
		if seen[k] {
			continue
		}
		if cur.GoneSince == nil {
			cur.GoneSince = &now
			cur.Stale = true
			if err := store.UpsertContainer(ctx, cur); err != nil {
				return nil, err
			}
			deltas = append(deltas, Delta{Op: OpUpdated, Kind: KindContainer, NodeID: nodeID, Container: ptrCtrView(cur)})
		} else {
			if err := store.DeleteContainer(ctx, cur.ID); err != nil {
				return nil, err
			}
			deltas = append(deltas, Delta{Op: OpRemoved, Kind: KindContainer, NodeID: nodeID, Container: ptrCtrView(cur)})
		}
	}
	return deltas, nil
}

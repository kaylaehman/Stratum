package inventory

import (
	"context"
	"log/slog"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/proxmox"
)

// enumProxmox enumerates the guests on ONLY the cluster member this Stratum node
// represents — not the whole cluster. proxmox1/proxmox2/proxmox3 are members of
// one cluster but each registered as its own Stratum node; the Proxmox API
// returns every member's guests regardless of which member answered, so
// enumerating all members from each node would surface every guest N times (one
// per Stratum node). We resolve the local member via cluster/status (the entry
// flagged local == 1) and enumerate only it.
//
// If the local member can't be determined (e.g. a permission gap on
// /cluster/status), we fall back to enumerating every online member — the old
// behavior — and log it, so a single-node deployment still works rather than
// going blank.
func enumProxmox(ctx context.Context, cl *proxmox.Client, nodeID string) ([]db.VM, error) {
	local, err := cl.LocalNodeName(ctx)
	if err != nil {
		slog.Warn("inventory: proxmox local node resolution failed; falling back to enumerating all online members (guests may duplicate across clustered Stratum nodes)",
			"node", nodeID, "error", err)
		return enumProxmoxAllMembers(ctx, cl, nodeID)
	}
	slog.Debug("inventory: proxmox local member resolved", "node", nodeID, "member", local)

	qemu, err := cl.QemuList(ctx, local)
	if err != nil {
		return nil, err
	}
	lxc, err := cl.LxcList(ctx, local)
	if err != nil {
		return nil, err
	}
	slog.Debug("inventory: proxmox local member guests", "node", nodeID, "member", local, "qemu", len(qemu), "lxc", len(lxc))

	out := make([]db.VM, 0, len(qemu)+len(lxc))
	for _, g := range append(qemu, lxc...) {
		out = append(out, db.VM{
			NodeID:      nodeID,
			Kind:        g.Kind,
			ProxmoxVMID: g.VMID,
			ProxmoxNode: local,
			Name:        g.Name,
			Status:      g.Status,
		})
	}
	// Confirmed auth but zero guests is almost always a token-permission gap:
	// without VM.Audit on /vms the Proxmox API returns an empty list (not an
	// error), so nothing above logs. Surface it so it's diagnosable.
	if len(out) == 0 {
		slog.Info("inventory: proxmox enumeration returned 0 guests; check the API token has VM.Audit on /vms (or a parent path)",
			"node", nodeID, "member", local)
	}
	return out, nil
}

// enumProxmoxAllMembers enumerates guests across every online cluster member. It
// is the fallback used only when the local member can't be resolved. A single
// member's list error is skipped (logged), not fatal, so one bad member doesn't
// block enumeration of the healthy ones — only a failure to list the cluster
// members themselves is a hard error.
func enumProxmoxAllMembers(ctx context.Context, cl *proxmox.Client, nodeID string) ([]db.VM, error) {
	members, err := cl.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	slog.Debug("inventory: proxmox members listed", "node", nodeID, "members", len(members))
	var out []db.VM
	for _, m := range members {
		if !m.Online {
			slog.Debug("inventory: proxmox member offline; skipping", "node", nodeID, "member", m.Name)
			continue // skip offline members (review §14)
		}
		qemu, err := cl.QemuList(ctx, m.Name)
		if err != nil {
			slog.Warn("inventory: qemu list failed for cluster member; skipping", "member", m.Name, "error", err)
			continue
		}
		lxc, err := cl.LxcList(ctx, m.Name)
		if err != nil {
			slog.Warn("inventory: lxc list failed for cluster member; skipping", "member", m.Name, "error", err)
			continue
		}
		slog.Debug("inventory: proxmox member guests", "node", nodeID, "member", m.Name, "qemu", len(qemu), "lxc", len(lxc))
		for _, g := range append(qemu, lxc...) {
			out = append(out, db.VM{
				NodeID:      nodeID,
				Kind:        g.Kind,
				ProxmoxVMID: g.VMID,
				ProxmoxNode: m.Name,
				Name:        g.Name,
				Status:      g.Status,
			})
		}
	}
	if len(out) == 0 {
		slog.Info("inventory: proxmox enumeration returned 0 guests; check the API token has VM.Audit on /vms (or a parent path)",
			"node", nodeID, "members", len(members))
	}
	return out, nil
}

// containerLister is the slice of the Docker client the enumerator depends on.
// Declaring it here (rather than taking *docker.Client) lets the poller inject a
// fake in tests — proving Docker enumeration runs independent of the SSH result.
type containerLister interface {
	ContainerList(ctx context.Context) ([]docker.ContainerInfo, error)
}

// containerInfo is the internal representation used between raw enumeration
// and DB conversion; it carries the HealthStatus field that ContainerInfo exposes.
type containerInfo = docker.ContainerInfo

// enumDockerRaw enumerates all containers (running and stopped) and returns
// the raw docker.ContainerInfo slice (which includes HealthStatus).
func enumDockerRaw(ctx context.Context, cl containerLister) ([]docker.ContainerInfo, error) {
	return cl.ContainerList(ctx)
}

// rawToDBContainers maps []docker.ContainerInfo to []db.Container for a node.
func rawToDBContainers(cs []docker.ContainerInfo, nodeID string) []db.Container {
	out := make([]db.Container, 0, len(cs))
	for _, c := range cs {
		out = append(out, db.Container{
			NodeID:         nodeID,
			DockerID:       c.ID,
			Name:           c.Name,
			Image:          c.Image,
			ImageID:        c.ImageID,
			Status:         c.State,
			ComposeProject: c.ComposeProject,
			ComposeService: c.ComposeService,
		})
	}
	return out
}

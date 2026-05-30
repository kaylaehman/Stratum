package inventory

import (
	"context"
	"log/slog"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/proxmox"
)

// enumProxmox enumerates all guests across every online cluster member. A
// single member's list error is skipped (logged), not fatal, so one bad member
// doesn't block enumeration of the healthy ones — only a failure to list the
// cluster members themselves is a hard error.
func enumProxmox(ctx context.Context, cl *proxmox.Client, nodeID string) ([]db.VM, error) {
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
	// Confirmed auth but zero guests is almost always a token-permission gap:
	// without VM.Audit on /vms the Proxmox API returns an empty list (not an
	// error), so nothing above logs. Surface it so it's diagnosable.
	if len(out) == 0 {
		slog.Info("inventory: proxmox enumeration returned 0 guests; check the API token has VM.Audit on /vms (or a parent path)",
			"node", nodeID, "members", len(members))
	}
	return out, nil
}

// enumDocker enumerates all containers (running and stopped) on a node.
func enumDocker(ctx context.Context, cl *docker.Client, nodeID string) ([]db.Container, error) {
	cs, err := cl.ContainerList(ctx)
	if err != nil {
		return nil, err
	}
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
		})
	}
	return out, nil
}

package inventory

import (
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// VMView is the JSON representation of a Proxmox guest in deltas and the tree.
type VMView struct {
	ID          string    `json:"id"`
	NodeID      string    `json:"node_id"`
	Kind        string    `json:"kind"`
	ProxmoxVMID int       `json:"proxmox_vmid"`
	ProxmoxNode string    `json:"proxmox_node"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	OSType      string    `json:"os_type,omitempty"`
	Stale       bool      `json:"stale"`
	LastSeen    time.Time `json:"last_seen"`
}

// ContainerView is the JSON representation of a Docker container.
type ContainerView struct {
	ID             string    `json:"id"`
	NodeID         string    `json:"node_id"`
	DockerID       string    `json:"docker_id"`
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	ImageID        string    `json:"image_id,omitempty"`
	Status         string    `json:"status"`
	ComposeProject string    `json:"compose_project,omitempty"`
	Stale          bool      `json:"stale"`
	LastSeen       time.Time `json:"last_seen"`
}

func FromVM(v db.VM) VMView {
	return VMView{
		ID: v.ID, NodeID: v.NodeID, Kind: v.Kind, ProxmoxVMID: v.ProxmoxVMID,
		ProxmoxNode: v.ProxmoxNode, Name: v.Name, Status: v.Status, OSType: v.OSType,
		Stale: v.Stale, LastSeen: v.LastSeen,
	}
}

func FromContainer(c db.Container) ContainerView {
	return ContainerView{
		ID: c.ID, NodeID: c.NodeID, DockerID: c.DockerID, Name: c.Name, Image: c.Image,
		ImageID: c.ImageID, Status: c.Status, ComposeProject: c.ComposeProject,
		Stale: c.Stale, LastSeen: c.LastSeen,
	}
}

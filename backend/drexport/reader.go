package drexport

import (
	"context"

	"github.com/KAE-Labs/stratum/backend/db"
)

// NodeReader provides the node slice needed by the service.
// Satisfied by *nodes.Service (which holds db.Store + cipher).
// Defined here so drexport has zero import on nodes (avoids circular deps).
type NodeReader interface {
	// List returns all node views (no secrets, no credentials).
	List(ctx context.Context) ([]NodeView, error)
}

// NodeView is a minimal projection of a node for the manifest.
// It mirrors nodes.NodeView but lives here so drexport has no dep on the
// nodes package — the nodes package satisfies the interface structurally.
type NodeView struct {
	ID           string
	Name         string
	Host         string
	Port         int
	Type         string
	OSType       string
	AuthMethod   string
	Capabilities map[string]any
	Status       string
}

// StoreReader is the narrow slice of db.Store that drexport needs.
// *sqlite.Store satisfies this automatically because it implements db.Store.
type StoreReader interface {
	ListNodes(ctx context.Context) ([]db.Node, error)
	ListSecretGroups(ctx context.Context) ([]db.SecretGroup, error)
	ListSecretKeysByGroup(ctx context.Context, groupID string) ([]db.SecretRow, error)
	ListStackEnvVars(ctx context.Context, nodeID, projectName string) ([]db.StackEnvRow, error)
	ListCerts(ctx context.Context) ([]db.CertInfo, error)
	ListContainersByNode(ctx context.Context, nodeID string) ([]db.Container, error)
}

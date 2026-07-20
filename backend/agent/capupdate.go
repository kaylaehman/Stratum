package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
)

// capsEnvelope is the shape of capabilities_json in the DB (mirrors the private
// type in backend/nodes to avoid an import cycle).
type capsEnvelope struct {
	capabilities.Set
	ProxmoxAuthStatus string `json:"proxmox_auth_status,omitempty"`
}

// probeAgentTimeout bounds a single agent liveness probe during capability
// update.
const probeAgentTimeout = 5 * time.Second

// UpdateAgentCapabilities iterates all nodes, probes each for an agent (at
// host:7750 with mTLS), and updates capabilities_json.agent when the result
// changes. It is safe to call concurrently and is designed to be called once
// at startup. A nil tlsCfg is a no-op.
func UpdateAgentCapabilities(ctx context.Context, store db.Store, tlsCfg *tls.Config, logger *slog.Logger) {
	if tlsCfg == nil {
		logger.Info("agent: capability update skipped (no TLS config)")
		return
	}

	nodes, err := store.ListNodes(ctx)
	if err != nil {
		logger.Warn("agent: capability update: list nodes", "error", err)
		return
	}

	for _, node := range nodes {
		caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))

		pctx, cancel := context.WithTimeout(ctx, probeAgentTimeout)
		reachable := ProbeAgent(pctx, node.ID, node.Host, tlsCfg)
		cancel()

		if reachable == caps.Agent {
			continue // no change
		}

		caps.Agent = reachable
		updated, err := mergeCaps(node.CapabilitiesJSON, caps)
		if err != nil {
			logger.Warn("agent: capability update: marshal caps", "node", node.ID, "error", err)
			continue
		}
		node.CapabilitiesJSON = updated
		if err := store.UpdateNode(ctx, node); err != nil {
			logger.Warn("agent: capability update: save node", "node", node.ID, "error", err)
			continue
		}
		logger.Info("agent: updated agent capability", "node", node.ID, "agent", reachable)
	}
}

// mergeCaps decodes the existing capabilities_json, applies new caps, and
// returns updated JSON. The ProxmoxAuthStatus and other fields are preserved.
func mergeCaps(existing string, newCaps capabilities.Set) (string, error) {
	var env capsEnvelope
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &env); err != nil {
			// If unmarshal fails, start fresh.
			env = capsEnvelope{}
		}
	}
	env.Set = newCaps
	b, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

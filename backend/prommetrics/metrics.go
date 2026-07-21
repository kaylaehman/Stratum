// Package prommetrics exposes Stratum's own operational metrics on a dedicated
// Prometheus registry.  It deliberately avoids editing peer packages — instead
// it accepts small getter functions injected at construction time so the caller
// (main.go) decides what to wire.
//
// Metrics exposed:
//
//	stratum_remediation_proposals_total{status}    — count of proposals by status (pull collector)
//	stratum_ws_clients                              — current WebSocket client count (pull collector)
//	stratum_ws_messages_dropped_total               — cumulative dropped WS messages (pull collector)
//	Go runtime + process collectors (defaults)
package prommetrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/KAE-Labs/stratum/backend/db"
)

// ProposalLister is the subset of db.Store used by the proposal collector.
// Using an interface here keeps the package testable without a real DB.
type ProposalLister interface {
	ListProposals(ctx context.Context, nodeID string) ([]db.RemediationProposal, error)
}

// HubStater returns live WebSocket hub statistics.
type HubStater interface {
	ClientCount() int
	Dropped() uint64
}

// Registry returns a new *prometheus.Registry pre-loaded with Stratum's
// internal collectors plus the standard Go runtime and process collectors.
// store and hub may be nil; in that case the corresponding collectors are
// omitted (degraded mode — useful in tests).
func Registry(store ProposalLister, hub HubStater) *prometheus.Registry {
	reg := prometheus.NewRegistry()

	// Standard Go runtime + process metrics (replaces default global registry).
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	if store != nil {
		reg.MustRegister(newProposalCollector(store))
	}

	if hub != nil {
		reg.MustRegister(newHubCollector(hub))
	}

	return reg
}

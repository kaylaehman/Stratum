package prommetrics_test

import (
	"context"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/prommetrics"
)

// --- fakes ----------------------------------------------------------------

type fakeStore struct {
	proposals []db.RemediationProposal
}

func (f *fakeStore) ListProposals(_ context.Context, _ string) ([]db.RemediationProposal, error) {
	return f.proposals, nil
}

type fakeHub struct {
	clients int
	dropped uint64
}

func (h *fakeHub) ClientCount() int    { return h.clients }
func (h *fakeHub) Dropped() uint64     { return h.dropped }

// --- helpers --------------------------------------------------------------

func gather(t *testing.T, store prommetrics.ProposalLister, hub prommetrics.HubStater) map[string]*dto.MetricFamily {
	t.Helper()
	reg := prommetrics.Registry(store, hub)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	out := make(map[string]*dto.MetricFamily, len(mfs))
	for _, mf := range mfs {
		out[mf.GetName()] = mf
	}
	return out
}

// --- tests ----------------------------------------------------------------

func TestRegistry_NilDeps(t *testing.T) {
	// Should not panic and should still expose Go runtime metrics.
	mfs := gather(t, nil, nil)
	if _, ok := mfs["go_goroutines"]; !ok {
		t.Error("expected go_goroutines metric")
	}
}

func TestProposalCollector_Counts(t *testing.T) {
	store := &fakeStore{proposals: []db.RemediationProposal{
		{Status: "proposed"},
		{Status: "proposed"},
		{Status: "executed"},
	}}
	mfs := gather(t, store, nil)
	mf, ok := mfs["stratum_remediation_proposals_total"]
	if !ok {
		t.Fatal("stratum_remediation_proposals_total not found")
	}
	counts := map[string]float64{}
	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "status" {
				counts[lp.GetValue()] = m.GetGauge().GetValue()
			}
		}
	}
	if counts["proposed"] != 2 {
		t.Errorf("proposed: want 2, got %v", counts["proposed"])
	}
	if counts["executed"] != 1 {
		t.Errorf("executed: want 1, got %v", counts["executed"])
	}
}

func TestProposalCollector_AllStatusesPresent(t *testing.T) {
	store := &fakeStore{}
	mfs := gather(t, store, nil)
	mf := mfs["stratum_remediation_proposals_total"]
	if mf == nil {
		t.Fatal("metric missing")
	}
	seen := map[string]bool{}
	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "status" {
				seen[lp.GetValue()] = true
			}
		}
	}
	for _, s := range []string{"proposed", "approved", "rejected", "executed", "failed"} {
		if !seen[s] {
			t.Errorf("status label %q missing from zero-valued metric", s)
		}
	}
}

func TestHubCollector(t *testing.T) {
	hub := &fakeHub{clients: 7, dropped: 42}
	mfs := gather(t, nil, hub)

	if v := mfs["stratum_ws_clients"].GetMetric()[0].GetGauge().GetValue(); v != 7 {
		t.Errorf("stratum_ws_clients: want 7, got %v", v)
	}
	if v := mfs["stratum_ws_messages_dropped_total"].GetMetric()[0].GetCounter().GetValue(); v != 42 {
		t.Errorf("stratum_ws_messages_dropped_total: want 42, got %v", v)
	}
}

func TestHubCollector_MetricNames(t *testing.T) {
	hub := &fakeHub{}
	mfs := gather(t, nil, hub)
	for _, name := range []string{"stratum_ws_clients", "stratum_ws_messages_dropped_total"} {
		if !strings.HasPrefix(name, "stratum_") {
			t.Errorf("metric %s lacks stratum_ prefix", name)
		}
		if _, ok := mfs[name]; !ok {
			t.Errorf("metric %s not found", name)
		}
	}
}

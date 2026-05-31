package prommetrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// knownStatuses is the exhaustive list of remediation proposal statuses defined
// in the data model.  We pre-initialise all label values so Grafana graphs show
// a zero series immediately instead of missing series until the first event.
var knownStatuses = []string{"proposed", "approved", "rejected", "executed", "failed"}

type proposalCollector struct {
	store ProposalLister
	desc  *prometheus.Desc
}

func newProposalCollector(store ProposalLister) *proposalCollector {
	return &proposalCollector{
		store: store,
		desc: prometheus.NewDesc(
			"stratum_remediation_proposals_total",
			"Number of remediation proposals grouped by status.",
			[]string{"status"},
			nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *proposalCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

// Collect implements prometheus.Collector.  It queries all proposals (nodeID=""
// = all nodes) and emits one counter per status label.  A query timeout of 5 s
// guards against a stuck DB; on error all counts are reported as 0 so scrapes
// never fail.
func (c *proposalCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	counts := make(map[string]int, len(knownStatuses))
	for _, s := range knownStatuses {
		counts[s] = 0
	}

	proposals, err := c.store.ListProposals(ctx, "" /* all nodes */)
	if err == nil {
		for _, p := range proposals {
			counts[p.Status]++
		}
	}

	for _, s := range knownStatuses {
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			float64(counts[s]),
			s,
		)
	}
}

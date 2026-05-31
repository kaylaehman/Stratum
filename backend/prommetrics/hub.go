package prommetrics

import "github.com/prometheus/client_golang/prometheus"

type hubCollector struct {
	hub          HubStater
	clientsDesc  *prometheus.Desc
	droppedDesc  *prometheus.Desc
}

func newHubCollector(hub HubStater) *hubCollector {
	return &hubCollector{
		hub: hub,
		clientsDesc: prometheus.NewDesc(
			"stratum_ws_clients",
			"Number of currently connected WebSocket clients.",
			nil, nil,
		),
		droppedDesc: prometheus.NewDesc(
			"stratum_ws_messages_dropped_total",
			"Cumulative count of WebSocket messages dropped due to slow consumers.",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *hubCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.clientsDesc
	ch <- c.droppedDesc
}

// Collect implements prometheus.Collector.
func (c *hubCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		c.clientsDesc,
		prometheus.GaugeValue,
		float64(c.hub.ClientCount()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.droppedDesc,
		prometheus.CounterValue,
		float64(c.hub.Dropped()),
	)
}

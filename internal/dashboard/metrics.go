package dashboard

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// dashboardMetrics holds all Prometheus gauges/counters for the dashboard.
type dashboardMetrics struct {
	nodesTotal     *prometheus.GaugeVec
	blockHeight    *prometheus.GaugeVec
	syncProgress   *prometheus.GaugeVec
	peersTotal     *prometheus.GaugeVec
	scrapeErrors   prometheus.Counter
	lastScrapeTime prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *dashboardMetrics {
	m := &dashboardMetrics{
		nodesTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bch_dashboard_nodes_total",
			Help: "Number of BlockchainNode CRs by phase.",
		}, []string{"phase"}),

		blockHeight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bch_dashboard_block_height",
			Help: "Latest block height reported by the node.",
		}, []string{"chain", "network", "node"}),

		syncProgress: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bch_dashboard_sync_progress",
			Help: "Sync progress percentage (0-100).",
		}, []string{"chain", "network", "node"}),

		peersTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bch_dashboard_peers_total",
			Help: "Number of connected p2p peers.",
		}, []string{"chain", "network", "node"}),

		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bch_dashboard_scrape_errors_total",
			Help: "Total number of errors encountered while scraping BlockchainNode CRs.",
		}),

		lastScrapeTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bch_dashboard_last_scrape_timestamp",
			Help: "Unix timestamp of the most recent successful scrape.",
		}),
	}

	reg.MustRegister(
		m.nodesTotal,
		m.blockHeight,
		m.syncProgress,
		m.peersTotal,
		m.scrapeErrors,
		m.lastScrapeTime,
	)
	return m
}

// update refreshes all gauges from the current node list.
func (m *dashboardMetrics) update(nodes []NodeInfo) {
	// Reset vec gauges so stale series disappear when nodes are deleted.
	m.nodesTotal.Reset()
	m.blockHeight.Reset()
	m.syncProgress.Reset()
	m.peersTotal.Reset()

	phases := map[string]float64{}
	for _, n := range nodes {
		phase := n.Phase
		if phase == "" {
			phase = "Pending"
		}
		phases[phase]++

		labels := prometheus.Labels{
			"chain":   n.Chain,
			"network": n.Network,
			"node":    n.Namespace + "/" + n.Name,
		}
		m.blockHeight.With(labels).Set(float64(n.BlockHeight))
		m.peersTotal.With(labels).Set(float64(n.PeersCount))
		m.syncProgress.With(labels).Set(parseSyncPercent(n.SyncProgress))
	}

	for phase, count := range phases {
		m.nodesTotal.With(prometheus.Labels{"phase": phase}).Set(count)
	}
}

// parseSyncPercent converts a string like "98.5%" to 98.5.
// Returns 100 when empty (fully synced) and 0 on parse errors.
func parseSyncPercent(s string) float64 {
	if s == "" {
		return 100
	}
	s = strings.TrimSuffix(s, "%")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

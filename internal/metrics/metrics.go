/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

var (
	blockHeight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "chainplane_node_block_height",
		Help: "Latest confirmed block height of the blockchain node",
	}, []string{"chain", "network", "node", "node_type"})

	syncProgress = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "chainplane_node_sync_progress",
		Help: "Sync progress percentage of the blockchain node (0-100)",
	}, []string{"chain", "network", "node"})

	peersCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "chainplane_node_peers_count",
		Help: "Number of connected peers",
	}, []string{"chain", "network", "node"})

	nodePhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "chainplane_node_phase",
		Help: "Current phase of the blockchain node (1 = active phase)",
	}, []string{"chain", "network", "node", "phase"})

	nodeRestarts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chainplane_node_restarts_total",
		Help: "Total number of auto-restarts triggered by the operator",
	}, []string{"chain", "network", "node", "reason"})

	degradedDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chainplane_node_degraded_duration_seconds",
		Help:    "Duration of degraded episodes before recovery or restart",
		Buckets: []float64{60, 300, 600, 900, 1800, 3600, 7200},
	}, []string{"chain", "network"})
)

func init() {
	metrics.Registry.MustRegister(blockHeight, syncProgress, peersCount, nodePhase, nodeRestarts, degradedDuration)
}

// RecordSyncStatus updates all sync-related metrics for a node.
func RecordSyncStatus(node *chainsv1alpha2.ChainInstance, status adapters.SyncStatus) {
	labels := prometheus.Labels{
		"chain":   string(node.Spec.Chain),
		"network": string(node.Spec.Network),
		"node":    node.Name,
	}

	blockHeight.With(prometheus.Labels{
		"chain":     string(node.Spec.Chain),
		"network":   string(node.Spec.Network),
		"node":      node.Name,
		"node_type": string(node.Spec.NodeType),
	}).Set(float64(status.CurrentBlock))

	syncProgress.With(labels).Set(status.Progress)
	peersCount.With(labels).Set(float64(status.Peers))
}

// RecordAutoRestart increments the restart counter and records degraded duration.
func RecordAutoRestart(node *chainsv1alpha2.ChainInstance, reason string, degradedSeconds float64) {
	nodeRestarts.With(prometheus.Labels{
		"chain":   string(node.Spec.Chain),
		"network": string(node.Spec.Network),
		"node":    node.Name,
		"reason":  reason,
	}).Inc()
	degradedDuration.With(prometheus.Labels{
		"chain":   string(node.Spec.Chain),
		"network": string(node.Spec.Network),
	}).Observe(degradedSeconds)
}

// RecordDegradedRecovery records the duration of a degraded episode that ended in recovery.
func RecordDegradedRecovery(node *chainsv1alpha2.ChainInstance, degradedSeconds float64) {
	degradedDuration.With(prometheus.Labels{
		"chain":   string(node.Spec.Chain),
		"network": string(node.Spec.Network),
	}).Observe(degradedSeconds)
}

// RecordPhase sets the phase gauge (1 for current phase, 0 for others).
func RecordPhase(node *chainsv1alpha2.ChainInstance, phase chainsv1alpha2.NodePhase) {
	phases := []chainsv1alpha2.NodePhase{
		chainsv1alpha2.NodePhasePending,
		chainsv1alpha2.NodePhaseSyncing,
		chainsv1alpha2.NodePhaseHealthy,
		chainsv1alpha2.NodePhaseDegraded,
		chainsv1alpha2.NodePhaseFailed,
	}

	for _, p := range phases {
		val := 0.0
		if p == phase {
			val = 1.0
		}
		nodePhase.With(prometheus.Labels{
			"chain":   string(node.Spec.Chain),
			"network": string(node.Spec.Network),
			"node":    node.Name,
			"phase":   string(p),
		}).Set(val)
	}
}

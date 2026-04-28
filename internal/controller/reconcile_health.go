/*
Copyright 2026.

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

package controller

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/metrics"
)

// ---------------------------------------------------------------------------
// Annotation keys used for health/status tracking
// ---------------------------------------------------------------------------

const (
	// annotationDegradedSince records the RFC 3339 timestamp when the node
	// first entered the Degraded phase. Used for auto-restart timeout.
	annotationDegradedSince = "nodes.chainplane.io/degraded-since"

	// annotationSyncStallSince / annotationLastSyncBlock track height
	// freeze detection across reconcile cycles.
	annotationSyncStallSince = "nodes.chainplane.io/sync-stall-since"
	annotationLastSyncBlock  = "nodes.chainplane.io/last-sync-block"

	// Progress-snapshot annotations for ETA calculation.
	annotationProgressSnapshotPct = "nodes.chainplane.io/progress-snapshot-pct"
	annotationProgressSnapshotAt  = "nodes.chainplane.io/progress-snapshot-at"

	// Block-rate snapshot annotations (fallback ETA method).
	annotationBlockSnapshotHeight = "nodes.chainplane.io/block-snapshot-height"
	annotationBlockSnapshotAt     = "nodes.chainplane.io/block-snapshot-at"

	// progressSnapshotInterval determines how frequently ETA snapshots are
	// refreshed.
	progressSnapshotInterval = 5 * time.Minute

	// healthCheckTimeout caps how long a single adapter health check may run
	// before being cancelled.
	healthCheckTimeout = 30 * time.Second

	// defaultDegradedTimeoutMinutes is the auto-restart delay when
	// spec.health.degradedTimeoutMinutes is not set.
	defaultDegradedTimeoutMinutes = int32(15)

	// defaultRPCPort is used when the user does not configure spec.rpc.port.
	defaultRPCPort = int32(8545)

	// maxETAHours caps the ETA value to avoid displaying absurdly large estimates.
	maxETAHours = 8760 // 1 year
)

// ---------------------------------------------------------------------------
// refreshStatus
// ---------------------------------------------------------------------------

// refreshStatus queries the node's health endpoint, detects stalls and
// block-lag, updates conditions and phase, and persists the result via a
// status patch.
func (r *BlockchainNodeReconciler) refreshStatus(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	adapter adapters.ChainAdapter,
) error {
	logger := log.FromContext(ctx)

	rpcPort := defaultRPCPort
	if node.Spec.RPC.Port > 0 {
		rpcPort = node.Spec.RPC.Port
	}
	rpcURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", node.Name, node.Namespace, rpcPort)

	hctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	status, err := adapter.HealthCheck(hctx, rpcURL)
	logger.Info("health-check result",
		"node", node.Name,
		"err", err,
		"isSyncing", status.IsSyncing,
		"stallExempt", status.StallExempt,
		"progress", status.Progress,
		"currentBlock", status.CurrentBlock,
	)

	base := client.MergeFrom(node.DeepCopy())
	now := metav1.NewTime(time.Now())

	if err != nil {
		r.applyDegradedFromError(node, err, now)
	} else {
		r.applyHealthyOrSyncing(ctx, node, status, now)
		metrics.RecordSyncStatus(node, status)
	}

	node.Status.ObservedGeneration = node.Generation

	// Handle degraded auto-restart or recovery tracking.
	logger.Info("pre-auto-restart", "node", node.Name, "phase", node.Status.Phase)
	if node.Status.Phase == nodesv1alpha1.NodePhaseDegraded {
		if restartErr := r.restartIfDegradedTooLong(ctx, node); restartErr != nil {
			logger.Error(restartErr, "auto-restart failed")
		}
	} else {
		r.clearDegradedTracking(ctx, node, logger)
	}

	return r.Status().Patch(ctx, node, base)
}

// ---------------------------------------------------------------------------
// Status application
// ---------------------------------------------------------------------------

// applyDegradedFromError marks the node Degraded when the health check fails.
func (r *BlockchainNodeReconciler) applyDegradedFromError(
	node *nodesv1alpha1.BlockchainNode,
	healthErr error,
	now metav1.Time,
) {
	node.Status.Phase = nodesv1alpha1.NodePhaseDegraded
	metrics.RecordPhase(node, nodesv1alpha1.NodePhaseDegraded)
	setCondition(node, nodesv1alpha1.ConditionDegraded, metav1.ConditionTrue, "HealthCheckFailed", healthErr.Error(), now)
	setCondition(node, nodesv1alpha1.ConditionReady, metav1.ConditionFalse, "HealthCheckFailed", "RPC health check failed", now)
}

// applyHealthyOrSyncing updates status based on the adapter's sync report.
func (r *BlockchainNodeReconciler) applyHealthyOrSyncing(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	status adapters.SyncStatus,
	now metav1.Time,
) {
	if status.CurrentBlock > 0 {
		node.Status.BlockHeight = status.CurrentBlock
	}
	if status.Peers > 0 {
		node.Status.PeersCount = status.Peers
	}
	ensureAnnotations(node)

	if status.IsSyncing {
		r.applySyncingStatus(ctx, node, status, now)
	} else {
		r.applyFullySyncedStatus(ctx, node, status, now)
	}
}

// applySyncingStatus handles the IsSyncing=true branch.
func (r *BlockchainNodeReconciler) applySyncingStatus(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	status adapters.SyncStatus,
	now metav1.Time,
) {
	logger := log.FromContext(ctx)

	// Enrich AVAX progress from pod logs during bootstrap.
	if node.Spec.Chain == nodesv1alpha1.ChainAvalanche && status.Progress == 0 {
		if pct := r.avaxBootstrapProgress(ctx, node); pct >= 0 {
			status.Progress = pct
		}
	}

	preserveProgress := status.Progress == 0 &&
		node.Status.SyncProgress != "" &&
		node.Status.SyncProgress != "0.0%"

	// Stall check (unless adapter signals exemption).
	logger.Info("stall-check gate", "node", node.Name, "stallExempt", status.StallExempt, "willCheck", !status.StallExempt)
	if !status.StallExempt {
		if stalled, msg := r.detectHeightStall(ctx, node, status.CurrentBlock, logger); stalled {
			r.markStalled(ctx, node, status.Progress, msg, now)
			return
		}
	}

	node.Status.Phase = nodesv1alpha1.NodePhaseSyncing
	if !preserveProgress {
		pct := status.Progress
		if pct > 99.9 {
			pct = 99.9
		}
		node.Status.SyncProgress = fmt.Sprintf("%.1f%%", pct)
		if eta := r.estimateSyncETA(ctx, node, status.Progress, status.CurrentBlock, now.Time); eta != "" {
			node.Status.SyncETA = eta
		}
	}

	setCondition(node, nodesv1alpha1.ConditionSyncing, metav1.ConditionTrue, "Syncing", fmt.Sprintf("%.1f%% complete", status.Progress), now)
	setCondition(node, nodesv1alpha1.ConditionReady, metav1.ConditionFalse, "Syncing", "Node is syncing", now)
	setCondition(node, nodesv1alpha1.ConditionDegraded, metav1.ConditionFalse, "Syncing", "Node is syncing", now)
}

// applyFullySyncedStatus handles the IsSyncing=false branch.
func (r *BlockchainNodeReconciler) applyFullySyncedStatus(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	status adapters.SyncStatus,
	now metav1.Time,
) {
	logger := log.FromContext(ctx)

	if stalled, msg := r.detectHeightStall(ctx, node, status.CurrentBlock, logger); stalled {
		node.Status.Phase = nodesv1alpha1.NodePhaseDegraded
		metrics.RecordPhase(node, nodesv1alpha1.NodePhaseDegraded)
		setCondition(node, nodesv1alpha1.ConditionDegraded, metav1.ConditionTrue, "HeightStall", msg, now)
		setCondition(node, nodesv1alpha1.ConditionReady, metav1.ConditionFalse, "HeightStall", msg, now)
		node.Status.SyncProgress = "stalled"
		node.Status.ObservedGeneration = node.Generation
		if err := r.restartIfDegradedTooLong(ctx, node); err != nil {
			logger.Error(err, "auto-restart failed for stalled healthy node")
		}
		return
	}

	node.Status.Phase = nodesv1alpha1.NodePhaseHealthy
	node.Status.SyncProgress = "100%"
	node.Status.SyncETA = ""
	setCondition(node, nodesv1alpha1.ConditionSyncing, metav1.ConditionFalse, "Synced", "Node is fully synced", now)

	// Clear stall tracking on genuine progress.
	delete(node.Annotations, annotationSyncStallSince)
	delete(node.Annotations, annotationLastSyncBlock)

	if lagging, lag := r.checkBlockLag(ctx, node, status.CurrentBlock); lagging {
		node.Status.Phase = nodesv1alpha1.NodePhaseDegraded
		node.Status.SyncProgress = fmt.Sprintf("lag: %d blocks", lag)
		metrics.RecordPhase(node, nodesv1alpha1.NodePhaseDegraded)
		msg := fmt.Sprintf("Block lag %d exceeds threshold", lag)
		setCondition(node, nodesv1alpha1.ConditionDegraded, metav1.ConditionTrue, "BlockLag", msg, now)
		setCondition(node, nodesv1alpha1.ConditionReady, metav1.ConditionFalse, "BlockLag", msg, now)
	} else {
		setCondition(node, nodesv1alpha1.ConditionDegraded, metav1.ConditionFalse, "Healthy", "Node is healthy", now)
		setCondition(node, nodesv1alpha1.ConditionReady, metav1.ConditionTrue, "Healthy", "Node is healthy and synced", now)
	}
}

// markStalled sets the node to Degraded with stall information.
func (r *BlockchainNodeReconciler) markStalled(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	progress float64,
	stallMsg string,
	now metav1.Time,
) {
	logger := log.FromContext(ctx)
	node.Status.Phase = nodesv1alpha1.NodePhaseDegraded
	metrics.RecordPhase(node, nodesv1alpha1.NodePhaseDegraded)
	setCondition(node, nodesv1alpha1.ConditionDegraded, metav1.ConditionTrue, "SyncStall", stallMsg, now)
	setCondition(node, nodesv1alpha1.ConditionReady, metav1.ConditionFalse, "SyncStall", stallMsg, now)
	setCondition(node, nodesv1alpha1.ConditionSyncing, metav1.ConditionTrue, "Syncing", fmt.Sprintf("%.1f%% complete (stalled)", progress), now)
	node.Status.SyncProgress = fmt.Sprintf("%.1f%% (stalled)", progress)
	node.Status.ObservedGeneration = node.Generation
	if err := r.restartIfDegradedTooLong(ctx, node); err != nil {
		logger.Error(err, "auto-restart failed for stalled node")
	}
}

// ---------------------------------------------------------------------------
// Height-stall detector
// ---------------------------------------------------------------------------

// detectHeightStall tracks block height via annotations and returns (stalled,
// message) when the height has been frozen for longer than the chain's stall
// threshold. Resets tracking whenever height advances.
func (r *BlockchainNodeReconciler) detectHeightStall(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	currentBlock int64,
	logger interface {
		Error(error, string, ...interface{})
	},
) (bool, string) {
	blockStr := fmt.Sprintf("%d", currentBlock)
	lastBlock := node.Annotations[annotationLastSyncBlock]
	stallSince := node.Annotations[annotationSyncStallSince]

	// Height advanced: reset tracking.
	if lastBlock != blockStr {
		node.Annotations[annotationLastSyncBlock] = blockStr
		delete(node.Annotations, annotationSyncStallSince)
		if err := r.persistAnnotations(ctx, node); err != nil {
			logger.Error(err, "persisting height-stall annotations")
		}
		return false, ""
	}

	// First time seeing height frozen: record the start time.
	if stallSince == "" {
		node.Annotations[annotationSyncStallSince] = time.Now().UTC().Format(time.RFC3339)
		if err := r.persistAnnotations(ctx, node); err != nil {
			logger.Error(err, "recording stall-since timestamp")
		}
		return false, ""
	}

	stallAt, err := time.Parse(time.RFC3339, stallSince)
	if err != nil {
		node.Annotations[annotationSyncStallSince] = time.Now().UTC().Format(time.RFC3339)
		_ = r.persistAnnotations(ctx, node)
		return false, ""
	}

	threshold := syncStallThreshold(node.Spec.Chain)
	if time.Since(stallAt) < threshold {
		return false, ""
	}

	msg := fmt.Sprintf("Height frozen at block %d for >%s (10 block-times for %s)",
		currentBlock, threshold.Round(time.Second), node.Spec.Chain)
	return true, msg
}

// ---------------------------------------------------------------------------
// Block-lag detector
// ---------------------------------------------------------------------------

// checkBlockLag fetches the public chain tip and returns (lagging, lag) when
// the node's height trails beyond the configured threshold.
func (r *BlockchainNodeReconciler) checkBlockLag(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	currentBlock int64,
) (bool, int64) {
	tip, err := fetchPublicTip(ctx, node.Spec.Chain)
	if err != nil {
		return false, 0
	}

	lag := tip - currentBlock
	if lag < 0 {
		lag = 0
	}
	return lag > lagThreshold(node), lag
}

// ---------------------------------------------------------------------------
// Degraded auto-restart
// ---------------------------------------------------------------------------

// restartIfDegradedTooLong records when degradation began and deletes the pod
// once the configured timeout expires so the StatefulSet recreates it.
func (r *BlockchainNodeReconciler) restartIfDegradedTooLong(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
) error {
	timeout := defaultDegradedTimeoutMinutes
	if node.Spec.Health.DegradedTimeoutMinutes != nil {
		timeout = *node.Spec.Health.DegradedTimeoutMinutes
	}
	if timeout == 0 {
		return nil
	}

	ensureAnnotations(node)

	since, exists := node.Annotations[annotationDegradedSince]
	if !exists {
		node.Annotations[annotationDegradedSince] = time.Now().UTC().Format(time.RFC3339)
		return r.persistAnnotations(ctx, node)
	}

	degradedAt, err := time.Parse(time.RFC3339, since)
	if err != nil {
		node.Annotations[annotationDegradedSince] = time.Now().UTC().Format(time.RFC3339)
		return r.persistAnnotations(ctx, node)
	}

	if time.Since(degradedAt) < time.Duration(timeout)*time.Minute {
		return nil
	}

	podName := fmt.Sprintf("%s-0", node.Name)
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: node.Namespace}, pod); err != nil {
		return client.IgnoreNotFound(err)
	}

	elapsed := time.Since(degradedAt).Seconds()
	log.FromContext(ctx).Info("auto-restarting degraded node pod",
		"pod", podName,
		"degraded-since", since,
		"timeout-minutes", timeout,
		"degraded-duration-seconds", elapsed,
	)

	if err := r.Delete(ctx, pod); err != nil {
		return fmt.Errorf("deleting pod %s/%s for auto-restart: %w", node.Namespace, podName, err)
	}

	metrics.RecordAutoRestart(node, "degraded-timeout", elapsed)

	delete(node.Annotations, annotationDegradedSince)
	return r.persistAnnotations(ctx, node)
}

// clearDegradedTracking removes the degraded-since annotation when the node
// recovers, and records the recovery duration for observability.
func (r *BlockchainNodeReconciler) clearDegradedTracking(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	logger interface {
		Error(error, string, ...interface{})
	},
) {
	sinceStr, hasSince := node.Annotations[annotationDegradedSince]
	if !hasSince {
		return
	}

	if degradedAt, err := time.Parse(time.RFC3339, sinceStr); err == nil {
		metrics.RecordDegradedRecovery(node, time.Since(degradedAt).Seconds())
	}

	ensureAnnotations(node)
	delete(node.Annotations, annotationDegradedSince)
	if err := r.persistAnnotations(ctx, node); err != nil {
		logger.Error(err, "clearing degraded-since annotation")
	}
}

// ---------------------------------------------------------------------------
// AVAX bootstrap progress
// ---------------------------------------------------------------------------

// avaxBootstrapProgress reads the last 200 log lines from the AVAX pod and
// extracts the most recent "pctComplete" value emitted by the bootstrapper.
// Returns -1 when progress cannot be determined.
func (r *BlockchainNodeReconciler) avaxBootstrapProgress(ctx context.Context, node *nodesv1alpha1.BlockchainNode) float64 {
	if r.KubeClientset == nil {
		return -1
	}
	podName := node.Name + "-0"
	tailLines := int64(200)
	stream, err := r.KubeClientset.CoreV1().Pods(node.Namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: mainContainerName,
		TailLines: &tailLines,
	}).Stream(ctx)
	if err != nil {
		return -1
	}
	defer stream.Close()

	var pct float64 = -1
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, `"pctComplete":`); idx >= 0 {
			rest := strings.TrimSpace(line[idx+len(`"pctComplete"`)+1:])
			if _, scanErr := fmt.Sscanf(rest, "%f", &pct); scanErr != nil {
				continue
			}
		}
	}
	return pct
}

// ---------------------------------------------------------------------------
// Sync ETA estimation
// ---------------------------------------------------------------------------

// estimateSyncETA estimates remaining sync time using a progress-rate method
// first, then falling back to block-rate when progress data is unavailable
// or has regressed (multi-stage syncs).
func (r *BlockchainNodeReconciler) estimateSyncETA(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	progress float64,
	currentBlock int64,
	now time.Time,
) string {
	if progress >= 100 {
		return ""
	}
	ensureAnnotations(node)

	// Try progress-based ETA.
	if eta := r.progressBasedETA(ctx, node, progress, now); eta != "" {
		return eta
	}

	// Fallback to block-rate ETA.
	return r.blockRateETA(ctx, node, currentBlock, now)
}

// progressBasedETA calculates ETA from the sync progress delta over time.
func (r *BlockchainNodeReconciler) progressBasedETA(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	progress float64,
	now time.Time,
) string {
	if progress <= 0 {
		return ""
	}

	snapshotPctStr := node.Annotations[annotationProgressSnapshotPct]
	snapshotAtStr := node.Annotations[annotationProgressSnapshotAt]

	var snapshotPct float64
	var snapshotAt time.Time
	hasSnapshot := false

	if snapshotPctStr != "" && snapshotAtStr != "" {
		if t, err := time.Parse(time.RFC3339, snapshotAtStr); err == nil {
			if p, err := strconv.ParseFloat(snapshotPctStr, 64); err == nil {
				snapshotPct = p
				snapshotAt = t
				hasSnapshot = true
			}
		}
	}

	// Detect progress regression (e.g. Erigon stage change).
	if hasSnapshot && (snapshotPct-progress) > 5.0 {
		delete(node.Annotations, annotationProgressSnapshotPct)
		delete(node.Annotations, annotationProgressSnapshotAt)
		return ""
	}

	if !hasSnapshot || now.Sub(snapshotAt) >= progressSnapshotInterval {
		node.Annotations[annotationProgressSnapshotPct] = strconv.FormatFloat(progress, 'f', 4, 64)
		node.Annotations[annotationProgressSnapshotAt] = now.Format(time.RFC3339)
		if err := r.persistAnnotations(ctx, node); err != nil {
			return ""
		}
		if !hasSnapshot {
			return ""
		}
	}

	elapsed := now.Sub(snapshotAt).Seconds()
	delta := progress - snapshotPct
	if elapsed < 60 || delta <= 0 {
		return ""
	}

	rate := delta / elapsed
	remaining := (100.0 - progress) / rate
	return humanDuration(time.Duration(remaining) * time.Second)
}

// blockRateETA estimates ETA from the block-catch-up rate.
func (r *BlockchainNodeReconciler) blockRateETA(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	currentBlock int64,
	now time.Time,
) string {
	if currentBlock <= 0 {
		return ""
	}

	tip, err := fetchPublicTip(ctx, node.Spec.Chain)
	if err != nil || tip <= currentBlock {
		return ""
	}

	heightStr := node.Annotations[annotationBlockSnapshotHeight]
	atStr := node.Annotations[annotationBlockSnapshotAt]

	var snapshotBlock int64
	var snapshotAt time.Time
	hasSnapshot := false

	if heightStr != "" && atStr != "" {
		if t, err := time.Parse(time.RFC3339, atStr); err == nil {
			if h, err := strconv.ParseInt(heightStr, 10, 64); err == nil {
				snapshotBlock = h
				snapshotAt = t
				hasSnapshot = true
			}
		}
	}

	if !hasSnapshot || now.Sub(snapshotAt) >= progressSnapshotInterval {
		node.Annotations[annotationBlockSnapshotHeight] = strconv.FormatInt(currentBlock, 10)
		node.Annotations[annotationBlockSnapshotAt] = now.Format(time.RFC3339)
		_ = r.persistAnnotations(ctx, node)
		return ""
	}

	elapsed := now.Sub(snapshotAt).Seconds()
	blockRate := float64(currentBlock-snapshotBlock) / elapsed
	if blockRate <= 0 {
		return ""
	}

	remaining := float64(tip-currentBlock) / blockRate
	eta := time.Duration(remaining) * time.Second
	if eta > time.Duration(maxETAHours)*time.Hour {
		return ""
	}
	return humanDuration(eta)
}

// ---------------------------------------------------------------------------
// Misc helpers
// ---------------------------------------------------------------------------

// ensureAnnotations guarantees the node has an initialised Annotations map.
func ensureAnnotations(node *nodesv1alpha1.BlockchainNode) {
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
}

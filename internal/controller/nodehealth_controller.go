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
	"context"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/health"
)

// ---------------------------------------------------------------------------
// Timing constants
// ---------------------------------------------------------------------------

const (
	// healthPollingInterval is how frequently the health controller re-checks
	// nodes that are not actively being replaced.
	healthPollingInterval = 60 * time.Second

	// replacementPollingInterval governs how often we poll during an active
	// blue/green replacement to advance through phases.
	replacementPollingInterval = 30 * time.Second
)

// ---------------------------------------------------------------------------
// Reconciler
// ---------------------------------------------------------------------------

// NodeHealthReconciler watches BlockchainNode resources and performs
// trigger-based pod replacement using the internal/health subsystem. It runs
// alongside BlockchainNodeReconciler without conflict: the main reconciler
// manages the desired state (StatefulSet, Service, ConfigMap) while this one
// handles health monitoring and blue/green replacement.
type NodeHealthReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Checker     *health.Checker
	Replacement *health.ReplacementManager
}

// SetupWithManager registers the NodeHealthReconciler with the controller manager.
func (r *NodeHealthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("nodehealth").
		For(&nodesv1alpha1.BlockchainNode{}).
		Complete(r)
}

// Reconcile performs a single health-check cycle for a BlockchainNode.
//
// The logic follows a state machine driven by replacement-phase annotations:
//  1. No replacement in progress: run trigger checks. If any fire, start replacement.
//  2. Replacement verifying: monitor the replacement pod. If healthy, complete.
//     If timed out, rollback.
//  3. Transient phases (Draining, Creating): requeue quickly to advance.
func (r *NodeHealthReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := slog.With("controller", "nodehealth", "node", req.Name, "namespace", req.Namespace)

	node := &nodesv1alpha1.BlockchainNode{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching BlockchainNode %s/%s: %w", req.Namespace, req.Name, err)
	}

	if !node.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if !isCheckablePhase(node.Status.Phase) {
		logger.Debug("skipping node not in a checkable phase", "phase", string(node.Status.Phase))
		return ctrl.Result{RequeueAfter: healthPollingInterval}, nil
	}

	if health.IsReplacementInProgress(node) {
		return r.advanceReplacement(ctx, node, logger)
	}
	return r.evaluateTriggers(ctx, node, logger)
}

// ---------------------------------------------------------------------------
// Phase gate
// ---------------------------------------------------------------------------

// isCheckablePhase returns true for phases where health monitoring is meaningful.
func isCheckablePhase(phase nodesv1alpha1.NodePhase) bool {
	switch phase {
	case nodesv1alpha1.NodePhaseHealthy, nodesv1alpha1.NodePhaseSyncing, nodesv1alpha1.NodePhaseDegraded:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Trigger evaluation
// ---------------------------------------------------------------------------

// evaluateTriggers runs all health triggers and starts a replacement when any
// trigger fires.
func (r *NodeHealthReconciler) evaluateTriggers(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	logger *slog.Logger,
) (ctrl.Result, error) {
	podName := node.Name + "-0"
	chain := mapChainToHealthType(node.Spec.Chain)

	logger.Info("running health trigger checks", "pod", podName, "chain", string(chain))

	result := r.Checker.CheckAll(ctx, podName, chain)
	if result.Healthy() {
		logger.Info("node is healthy, no triggers active")
		return ctrl.Result{RequeueAfter: healthPollingInterval}, nil
	}

	triggered := result.TriggeredNames()
	names := make([]string, len(triggered))
	for i, n := range triggered {
		names[i] = string(n)
	}
	logger.Warn("health triggers fired, initiating replacement", "pod", podName, "triggers", names)

	if err := r.Replacement.StartReplacement(ctx, node); err != nil {
		logger.Error("failed to start replacement", "error", err)
		return ctrl.Result{RequeueAfter: healthPollingInterval}, fmt.Errorf("starting replacement for %s/%s: %w", node.Namespace, node.Name, err)
	}

	return ctrl.Result{RequeueAfter: replacementPollingInterval}, nil
}

// ---------------------------------------------------------------------------
// Replacement state machine
// ---------------------------------------------------------------------------

// advanceReplacement handles ongoing replacement state transitions.
func (r *NodeHealthReconciler) advanceReplacement(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	logger *slog.Logger,
) (ctrl.Result, error) {
	phase := health.GetReplacementPhase(node)
	logger.Info("replacement in progress", "phase", string(phase))

	switch phase {
	case health.ReplacementVerifying:
		return r.handleVerifying(ctx, node, logger)

	case health.ReplacementDraining, health.ReplacementCreating:
		logger.Info("replacement in transient phase, requeueing", "phase", string(phase))
		return ctrl.Result{RequeueAfter: replacementPollingInterval}, nil

	case health.ReplacementRollingBack:
		logger.Warn("found stale rollback phase, retrying rollback")
		if err := r.Replacement.RollbackReplacement(ctx, node); err != nil {
			logger.Error("retry rollback failed", "error", err)
			return ctrl.Result{RequeueAfter: healthPollingInterval}, fmt.Errorf("retrying rollback for %s/%s: %w", node.Namespace, node.Name, err)
		}
		return ctrl.Result{RequeueAfter: healthPollingInterval}, nil

	default:
		logger.Warn("unknown replacement phase, clearing via rollback", "phase", string(phase))
		if err := r.Replacement.RollbackReplacement(ctx, node); err != nil {
			logger.Error("rollback for unknown phase failed", "error", err)
		}
		return ctrl.Result{RequeueAfter: healthPollingInterval}, nil
	}
}

// handleVerifying monitors a replacement pod that is in the verification phase.
func (r *NodeHealthReconciler) handleVerifying(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	logger *slog.Logger,
) (ctrl.Result, error) {
	ready, err := r.Replacement.MonitorReplacement(ctx, node)
	if err != nil {
		logger.Error("replacement monitoring failed, rolling back", "error", err)
		if rbErr := r.Replacement.RollbackReplacement(ctx, node); rbErr != nil {
			logger.Error("rollback also failed", "error", rbErr)
			return ctrl.Result{RequeueAfter: healthPollingInterval}, fmt.Errorf("rollback after monitoring failure for %s/%s: %w", node.Namespace, node.Name, rbErr)
		}
		return ctrl.Result{RequeueAfter: healthPollingInterval}, nil
	}

	if !ready {
		logger.Info("replacement pod not yet ready, rechecking")
		return ctrl.Result{RequeueAfter: replacementPollingInterval}, nil
	}

	logger.Info("replacement pod verified healthy, completing replacement")
	if err := r.Replacement.CompleteReplacement(ctx, node); err != nil {
		logger.Error("failed to complete replacement", "error", err)
		return ctrl.Result{RequeueAfter: replacementPollingInterval}, fmt.Errorf("completing replacement for %s/%s: %w", node.Namespace, node.Name, err)
	}

	logger.Info("replacement completed successfully")
	return ctrl.Result{RequeueAfter: healthPollingInterval}, nil
}

// ---------------------------------------------------------------------------
// Chain type mapping
// ---------------------------------------------------------------------------

// mapChainToHealthType converts the CRD Chain enum to the health package's
// BlockchainType.
func mapChainToHealthType(chain nodesv1alpha1.Chain) health.BlockchainType {
	switch chain {
	case nodesv1alpha1.ChainEthereum, nodesv1alpha1.ChainEthereumArchive:
		return health.BlockchainEthereum
	case nodesv1alpha1.ChainSolana:
		return health.BlockchainSolana
	default:
		return health.BlockchainEthereum
	}
}

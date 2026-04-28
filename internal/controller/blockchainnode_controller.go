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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/adapters"
)

// ---------------------------------------------------------------------------
// Timing constants
// ---------------------------------------------------------------------------

const (
	// reconcileInterval governs how often the controller re-evaluates
	// a node after a successful reconciliation pass.
	reconcileInterval = 30 * time.Second
)

// ---------------------------------------------------------------------------
// Reconciler
// ---------------------------------------------------------------------------

// BlockchainNodeReconciler drives the full lifecycle of a BlockchainNode CR:
// StatefulSet, Service, ConfigMap, health monitoring, and rolling upgrades.
//
// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete
type BlockchainNodeReconciler struct {
	client.Client
	APIReader     client.Reader
	Scheme        *runtime.Scheme
	KubeClientset *kubernetes.Clientset
}

// SetupWithManager registers the reconciler and declares owned resources so
// that changes to child objects trigger a reconciliation of the parent CR.
func (r *BlockchainNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodesv1alpha1.BlockchainNode{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

// Reconcile is the top-level entry point for a single reconciliation pass.
// It follows a deterministic sequence: fetch CR, resolve adapter, handle
// deletion, ensure finalizer, reconcile child resources, run upgrade logic,
// and finally refresh the observed status.
func (r *BlockchainNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &nodesv1alpha1.BlockchainNode{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching BlockchainNode %s/%s: %w", req.Namespace, req.Name, err)
	}

	adapter, ok := adapters.Get(node.Spec.Chain)
	if !ok {
		logger.Error(fmt.Errorf("no adapter for chain %s", node.Spec.Chain), "unsupported chain")
		return r.patchPhase(ctx, node, nodesv1alpha1.NodePhaseFailed)
	}

	// Handle deletion before anything else.
	if !node.DeletionTimestamp.IsZero() {
		return r.teardown(ctx, node)
	}

	// Ensure the finalizer is present.
	if !controllerutil.ContainsFinalizer(node, nodesv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(node, nodesv1alpha1.FinalizerName)
		if err := r.Update(ctx, node); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer to %s/%s: %w", node.Namespace, node.Name, err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile owned resources in dependency order.
	cfgHash, err := r.ensureConfigMap(ctx, node, adapter)
	if err != nil {
		logger.Error(err, "reconciling ConfigMap")
		return ctrl.Result{}, err
	}

	if err := r.ensureStatefulSet(ctx, node, adapter, cfgHash); err != nil {
		logger.Error(err, "reconciling StatefulSet")
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, node, adapter); err != nil {
		logger.Error(err, "reconciling Service")
		return ctrl.Result{}, err
	}

	if err := r.ensurePodMonitor(ctx, node, adapter); err != nil {
		logger.Error(err, "reconciling PodMonitor")
		return ctrl.Result{}, err
	}

	if err := r.reconcileUpgrade(ctx, node); err != nil {
		logger.Error(err, "reconciling upgrade")
		return ctrl.Result{}, err
	}

	if err := r.refreshStatus(ctx, node, adapter); err != nil {
		logger.Error(err, "refreshing status")
	}

	return ctrl.Result{RequeueAfter: reconcileInterval}, nil
}

// ---------------------------------------------------------------------------
// Deletion
// ---------------------------------------------------------------------------

// teardown performs a graceful scale-down and removes the finalizer once pods
// have terminated.
func (r *BlockchainNodeReconciler) teardown(ctx context.Context, node *nodesv1alpha1.BlockchainNode) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	sts := &appsv1.StatefulSet{}
	key := client.ObjectKeyFromObject(node)
	if err := r.Get(ctx, key, sts); err == nil {
		if sts.Spec.Replicas == nil || *sts.Spec.Replicas > 0 {
			zero := int32(0)
			sts.Spec.Replicas = &zero
			if err := r.Update(ctx, sts); err != nil {
				return ctrl.Result{}, fmt.Errorf("scaling StatefulSet %s/%s to zero: %w", node.Namespace, node.Name, err)
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		if sts.Status.ReadyReplicas > 0 {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	if controllerutil.ContainsFinalizer(node, nodesv1alpha1.FinalizerName) {
		controllerutil.RemoveFinalizer(node, nodesv1alpha1.FinalizerName)
		if err := r.Update(ctx, node); err != nil {
			return ctrl.Result{}, fmt.Errorf("removing finalizer from %s/%s: %w", node.Namespace, node.Name, err)
		}
		logger.Info("finalizer removed, node deleted gracefully")
	}

	return ctrl.Result{}, nil
}

// ---------------------------------------------------------------------------
// Phase patch
// ---------------------------------------------------------------------------

// patchPhase is a convenience wrapper that patches only the status phase.
func (r *BlockchainNodeReconciler) patchPhase(ctx context.Context, node *nodesv1alpha1.BlockchainNode, phase nodesv1alpha1.NodePhase) (ctrl.Result, error) {
	base := client.MergeFrom(node.DeepCopy())
	node.Status.Phase = phase
	return ctrl.Result{}, r.Status().Patch(ctx, node, base)
}

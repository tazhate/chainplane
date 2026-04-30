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

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
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

// ChainInstanceReconciler drives the full lifecycle of a ChainInstance CR:
// StatefulSet, Service, ConfigMap, health monitoring, and rolling upgrades.
type ChainInstanceReconciler struct {
	client.Client
	APIReader     client.Reader
	Scheme        *runtime.Scheme
	KubeClientset *kubernetes.Clientset
}

// SetupWithManager registers the reconciler and declares owned resources so
// that changes to child objects trigger a reconciliation of the parent CR.
func (r *ChainInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chainsv1alpha2.ChainInstance{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *ChainInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &chainsv1alpha2.ChainInstance{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ChainInstance %s/%s: %w", req.Namespace, req.Name, err)
	}

	adapter, ok := adapters.Get(node.Spec.Chain)
	if !ok {
		logger.Error(fmt.Errorf("no adapter for chain %s", node.Spec.Chain), "unsupported chain")
		return r.patchPhase(ctx, node, chainsv1alpha2.NodePhaseFailed)
	}

	// Strip legacy finalizer left on CRs created by older operator versions;
	// owner-reference cascade now handles graceful pod shutdown.
	if controllerutil.ContainsFinalizer(node, chainsv1alpha2.FinalizerName) {
		controllerutil.RemoveFinalizer(node, chainsv1alpha2.FinalizerName)
		if err := r.Update(ctx, node); err != nil {
			return ctrl.Result{}, fmt.Errorf("removing legacy finalizer from %s/%s: %w", node.Namespace, node.Name, err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !node.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
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
// Phase patch
// ---------------------------------------------------------------------------

// patchPhase is a convenience wrapper that patches only the status phase.
func (r *ChainInstanceReconciler) patchPhase(ctx context.Context, node *chainsv1alpha2.ChainInstance, phase chainsv1alpha2.NodePhase) (ctrl.Result, error) {
	base := client.MergeFrom(node.DeepCopy())
	node.Status.Phase = phase
	return ctrl.Result{}, r.Status().Patch(ctx, node, base)
}

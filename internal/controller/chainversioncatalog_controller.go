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
package controller

import (
	"context"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/registry"
)

const (
	defaultCheckInterval = 6 * time.Hour
	maxRecentTags        = 10
)

type ChainVersionCatalogReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=chains.chainplane.io,resources=chainversioncatalogs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=chains.chainplane.io,resources=chainversioncatalogs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=chains.chainplane.io,resources=chainversioncatalogs/finalizers,verbs=update

func (r *ChainVersionCatalogReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	catalog := &chainsv1alpha2.ChainVersionCatalog{}
	if err := r.Get(ctx, req.NamespacedName, catalog); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	adapter, ok := adapters.Get(catalog.Spec.Chain)
	if !ok {
		logger.Info("adapter not found", "chain", catalog.Spec.Chain)
		setReachableCondition(catalog, metav1.ConditionFalse, "AdapterNotFound",
			"no adapter registered for chain "+string(catalog.Spec.Chain))
		if err := r.Status().Update(ctx, catalog); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	vp, ok := adapter.(adapters.VersionProvider)
	if !ok {
		logger.Info("adapter does not implement VersionProvider", "chain", catalog.Spec.Chain)
		setReachableCondition(catalog, metav1.ConditionFalse, "VersionProviderNotImplemented",
			"adapter for chain "+string(catalog.Spec.Chain)+" does not implement VersionProvider")
		if err := r.Status().Update(ctx, catalog); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	policy := vp.VersionPolicy()

	regClient, err := registry.NewClient(policy.Registry)
	if err != nil {
		logger.Error(err, "failed to create registry client", "registry", policy.Registry)
		setReachableCondition(catalog, metav1.ConditionFalse, "RegistryError", err.Error())
		if err2 := r.Status().Update(ctx, catalog); err2 != nil {
			return ctrl.Result{}, err2
		}
		return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	entries, err := regClient.LatestTags(fetchCtx, policy, 25)
	if err != nil {
		logger.Error(err, "failed to fetch tags", "chain", catalog.Spec.Chain)
		setReachableCondition(catalog, metav1.ConditionFalse, "RegistryError", err.Error())
		if err2 := r.Status().Update(ctx, catalog); err2 != nil {
			return ctrl.Result{}, err2
		}
		return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
	}

	if len(entries) == 0 {
		logger.Info("no matching tags found", "chain", catalog.Spec.Chain)
		setReachableCondition(catalog, metav1.ConditionFalse, "NoMatchingTags",
			"registry returned no tags matching the policy pattern")
		if err := r.Status().Update(ctx, catalog); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	best := findLatestTag(entries, policy)

	sorted := make([]registry.TagEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return registry.IsNewer(sorted[i].Tag, sorted[j].Tag, policy.TagPrefix)
	})

	now := metav1.Now()
	catalog.Status.LatestTag = best.Tag
	catalog.Status.LatestDigest = best.Digest
	catalog.Status.CheckedAt = &now

	if best.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, best.PublishedAt); err == nil {
			mt := metav1.NewTime(t)
			catalog.Status.PublishedAt = &mt
		}
	}

	limit := maxRecentTags
	if len(sorted) < limit {
		limit = len(sorted)
	}
	recentTags := make([]chainsv1alpha2.TagInfo, 0, limit)
	for _, e := range sorted[:limit] {
		ti := chainsv1alpha2.TagInfo{Tag: e.Tag}
		if e.PublishedAt != "" {
			if t, err := time.Parse(time.RFC3339, e.PublishedAt); err == nil {
				mt := metav1.NewTime(t)
				ti.PublishedAt = &mt
			}
		}
		recentTags = append(recentTags, ti)
	}
	catalog.Status.RecentTags = recentTags

	setReachableCondition(catalog, metav1.ConditionTrue, "RegistryPolled",
		"successfully fetched tags from registry")

	if err := r.Status().Update(ctx, catalog); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("catalog updated", "chain", catalog.Spec.Chain, "latestTag", best.Tag)

	interval := parseCheckInterval(catalog.Spec.CheckInterval)
	return ctrl.Result{RequeueAfter: interval}, nil
}

func (r *ChainVersionCatalogReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chainsv1alpha2.ChainVersionCatalog{}).
		Complete(r)
}

func parseCheckInterval(s string) time.Duration {
	if s == "" {
		return defaultCheckInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return defaultCheckInterval
	}
	return d
}

func findLatestTag(entries []registry.TagEntry, policy adapters.ChainVersionPolicy) registry.TagEntry {
	if len(entries) == 0 {
		return registry.TagEntry{}
	}
	best := entries[0]
	for _, e := range entries[1:] {
		if registry.IsNewer(e.Tag, best.Tag, policy.TagPrefix) {
			best = e
		}
	}
	return best
}

func setReachableCondition(catalog *chainsv1alpha2.ChainVersionCatalog, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&catalog.Status.Conditions, metav1.Condition{
		Type:               chainsv1alpha2.ConditionRegistryReachable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: catalog.Generation,
	})
}

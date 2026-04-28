package controller

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/adapters"
)

var podMonitorGVK = schema.GroupVersionKind{
	Group:   "monitoring.coreos.com",
	Version: "v1",
	Kind:    "PodMonitor",
}

// ensurePodMonitor creates or updates a PodMonitor when monitoring is enabled
// and the adapter exposes a "metrics" named port. If either condition is false,
// any existing PodMonitor is removed.
//
// Errors caused by the prometheus-operator CRDs not being installed are logged
// as informational messages rather than hard failures, so the operator remains
// functional in clusters without Prometheus Operator.
func (r *BlockchainNodeReconciler) ensurePodMonitor(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	adapter adapters.ChainAdapter,
) error {
	logger := log.FromContext(ctx)

	_, hasMetrics := adapterMetricsPort(adapter, node.Spec)
	want := node.Spec.Monitoring.PodMonitor.Enabled && hasMetrics

	if !want {
		return r.deletePodMonitor(ctx, node)
	}

	pm := &unstructured.Unstructured{}
	pm.SetGroupVersionKind(podMonitorGVK)
	pm.SetName(node.Name)
	pm.SetNamespace(node.Namespace)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pm, func() error {
		applyPodMonitorSpec(pm, node)
		return controllerutil.SetControllerReference(node, pm, r.Scheme)
	})
	if err != nil {
		if isPodMonitorCRDMissing(err) {
			logger.Info("PodMonitor CRD not installed, skipping", "node", node.Name)
			return nil
		}
		return fmt.Errorf("upserting PodMonitor for %s/%s: %w", node.Namespace, node.Name, err)
	}
	return nil
}

// deletePodMonitor removes the PodMonitor for the given node if it exists.
func (r *BlockchainNodeReconciler) deletePodMonitor(ctx context.Context, node *nodesv1alpha1.BlockchainNode) error {
	pm := &unstructured.Unstructured{}
	pm.SetGroupVersionKind(podMonitorGVK)
	pm.SetName(node.Name)
	pm.SetNamespace(node.Namespace)

	if err := r.Delete(ctx, pm); err != nil {
		if errors.IsNotFound(err) || isPodMonitorCRDMissing(err) {
			return nil
		}
		return fmt.Errorf("deleting PodMonitor for %s/%s: %w", node.Namespace, node.Name, err)
	}
	return nil
}

// applyPodMonitorSpec writes the desired PodMonitor spec into the unstructured object.
// Called both on Create and Update so the function is idempotent.
func applyPodMonitorSpec(pm *unstructured.Unstructured, node *nodesv1alpha1.BlockchainNode) {
	spec := node.Spec.Monitoring.PodMonitor

	endpoint := map[string]interface{}{
		"port": "metrics",
		"path": "/metrics",
	}
	if spec.Interval != "" {
		endpoint["interval"] = spec.Interval
	}

	labels := coreLabels(node)
	for k, v := range spec.Labels {
		labels[k] = v
	}
	pm.SetLabels(labels)

	_ = unstructured.SetNestedField(pm.Object, selectorLabelsUnstructured(node), "spec", "selector", "matchLabels")
	_ = unstructured.SetNestedSlice(pm.Object, []interface{}{endpoint}, "spec", "podMetricsEndpoints")
}

// selectorLabelsUnstructured returns selectorLabels as map[string]interface{} for unstructured use.
func selectorLabelsUnstructured(node *nodesv1alpha1.BlockchainNode) map[string]interface{} {
	m := make(map[string]interface{}, len(selectorLabels(node)))
	for k, v := range selectorLabels(node) {
		m[k] = v
	}
	return m
}

// adapterMetricsPort returns the port number of the "metrics" named port from
// the adapter's ContainerPorts, or (0, false) if none is defined.
func adapterMetricsPort(adapter adapters.ChainAdapter, spec nodesv1alpha1.BlockchainNodeSpec) (int32, bool) {
	for _, p := range adapter.ContainerPorts(spec) {
		if p.Name == "metrics" {
			return p.ContainerPort, true
		}
	}
	return 0, false
}

// isPodMonitorCRDMissing returns true when the error indicates that the
// monitoring.coreos.com CRDs are not installed in the cluster.
func isPodMonitorCRDMissing(err error) bool {
	if apimeta.IsNoMatchError(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "no kind is registered") ||
		strings.Contains(msg, "the server could not find the requested resource")
}

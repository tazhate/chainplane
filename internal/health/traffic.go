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
package health

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Timing constants for the label-based traffic manager.
const (
	labelPropagationDelay = 5 * time.Second
	defaultDrainTimeout   = 30 * time.Second
	drainLogInterval      = 5 * time.Second
)

// LabelBasedTrafficManager implements Drainer by toggling the
// chains.chainplane.io/ready label on pods. Services must include
// "chains.chainplane.io/ready=true" in their selector for this to control
// endpoint membership.
type LabelBasedTrafficManager struct {
	kube client.Client
}

// NewLabelBasedTrafficManager returns a Drainer backed by label patching.
func NewLabelBasedTrafficManager(kube client.Client) *LabelBasedTrafficManager {
	return &LabelBasedTrafficManager{kube: kube}
}

// compile-time interface check.
var _ Drainer = (*LabelBasedTrafficManager)(nil)

// Drain removes the pod from service endpoints by setting ready=false and
// then waits for the remaining drain period so active connections can
// complete. If the pod is already gone, Drain returns nil.
func (t *LabelBasedTrafficManager) Drain(ctx context.Context, podName, namespace string) error {
	log := slog.With("op", "drain", "pod", podName, "ns", namespace)
	log.InfoContext(ctx, "starting traffic drain")

	if err := t.setPodLabel(ctx, podName, namespace, LabelReady, "false"); err != nil {
		if apierrors.IsNotFound(err) {
			log.WarnContext(ctx, "pod not found, skipping drain")
			return nil
		}
		return fmt.Errorf("mark pod not-ready: %w", err)
	}
	log.InfoContext(ctx, "pod removed from service endpoints")

	t.logDrainStatus(ctx, podName, namespace)

	duration := defaultDrainTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < duration {
			duration = remaining
		}
	}

	log.InfoContext(ctx, "waiting for connections to drain",
		"drain_seconds", int(duration.Seconds()))
	return waitWithProgress(ctx, duration, log)
}

// SwitchTraffic atomically moves traffic from the old pod to the new pod:
//  1. Set old pod ready=false (removes from service).
//  2. Wait for label propagation.
//  3. Set new pod ready=true (adds to service).
func (t *LabelBasedTrafficManager) SwitchTraffic(ctx context.Context, oldPod, newPod, namespace string) error {
	log := slog.With("op", "switch", "old", oldPod, "new", newPod, "ns", namespace)
	log.InfoContext(ctx, "switching traffic")

	// Step 1: Remove old pod.
	if err := t.setPodLabel(ctx, oldPod, namespace, LabelReady, "false"); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("remove old pod from service: %w", err)
		}
		log.WarnContext(ctx, "old pod not found, continuing")
	}

	// Step 2: Wait for propagation.
	select {
	case <-time.After(labelPropagationDelay):
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during propagation wait: %w", ctx.Err())
	}

	// Step 3: Add new pod.
	if err := t.setPodLabel(ctx, newPod, namespace, LabelReady, "true"); err != nil {
		return fmt.Errorf("add new pod to service: %w", err)
	}

	log.InfoContext(ctx, "traffic switch completed")
	return nil
}

// ValidateTraffic verifies that the pod is receiving traffic by checking:
//  1. Pod has the ready=true label.
//  2. Pod IP appears in matching service endpoint subsets.
func (t *LabelBasedTrafficManager) ValidateTraffic(ctx context.Context, podName, namespace string) (bool, error) {
	log := slog.With("op", "validate", "pod", podName, "ns", namespace)

	var pod corev1.Pod
	if err := t.kube.Get(ctx, types.NamespacedName{
		Name: podName, Namespace: namespace,
	}, &pod); err != nil {
		return false, fmt.Errorf("get pod %s: %w", podName, err)
	}

	if pod.Labels[LabelReady] != "true" {
		log.WarnContext(ctx, "pod missing ready=true label",
			"current", pod.Labels[LabelReady])
		return false, nil
	}

	podIP := pod.Status.PodIP
	if podIP == "" {
		log.WarnContext(ctx, "pod has no IP assigned")
		return false, nil
	}

	services, err := t.matchingServices(ctx, namespace, pod.Labels)
	if err != nil {
		return false, fmt.Errorf("find matching services: %w", err)
	}
	if len(services) == 0 {
		log.WarnContext(ctx, "no matching services, skipping endpoint check")
		return true, nil
	}

	for _, svc := range services {
		found, err := t.podInEndpoints(ctx, svc.Name, namespace, podIP)
		if err != nil {
			log.WarnContext(ctx, "endpoint check failed",
				"service", svc.Name, "error", err)
			continue
		}
		if !found {
			log.WarnContext(ctx, "pod IP not in endpoints",
				"service", svc.Name, "pod_ip", podIP)
			return false, nil
		}
	}

	log.InfoContext(ctx, "traffic validation passed",
		"services_checked", len(services))
	return true, nil
}

// --- unexported helpers ---

// setPodLabel applies a merge patch to set a single label on a pod.
func (t *LabelBasedTrafficManager) setPodLabel(
	ctx context.Context, name, namespace, key, value string,
) error {
	var pod corev1.Pod
	if err := t.kube.Get(ctx, types.NamespacedName{
		Name: name, Namespace: namespace,
	}, &pod); err != nil {
		return err
	}
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[key] = value
	return t.kube.Patch(ctx, &pod, patch)
}

// logDrainStatus logs pod details for observability during drain.
func (t *LabelBasedTrafficManager) logDrainStatus(ctx context.Context, name, namespace string) {
	var pod corev1.Pod
	if err := t.kube.Get(ctx, types.NamespacedName{
		Name: name, Namespace: namespace,
	}, &pod); err != nil {
		slog.WarnContext(ctx, "cannot fetch pod for drain logging", "error", err)
		return
	}

	slog.InfoContext(ctx, "pod status during drain",
		"pod", name, "phase", pod.Status.Phase,
		"ip", pod.Status.PodIP, "started", pod.Status.StartTime)

	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]
		slog.InfoContext(ctx, "container during drain",
			"container", cs.Name, "ready", cs.Ready, "restarts", cs.RestartCount)
	}
}

// waitWithProgress blocks for the given duration, logging periodic progress.
func waitWithProgress(ctx context.Context, duration time.Duration, log *slog.Logger) error {
	deadline := time.Now().Add(duration)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		tick := drainLogInterval
		if remaining < tick {
			tick = remaining
		}

		select {
		case <-time.After(tick):
			if left := time.Until(deadline); left > 0 {
				log.InfoContext(ctx, "draining...",
					"remaining_seconds", int(left.Seconds()))
			}
		case <-ctx.Done():
			log.WarnContext(ctx, "drain interrupted")
			return ctx.Err()
		}
	}

	log.InfoContext(ctx, "drain complete")
	return nil
}

// matchingServices returns services whose selector is a subset of the pod labels.
func (t *LabelBasedTrafficManager) matchingServices(
	ctx context.Context, namespace string, podLabels map[string]string,
) ([]corev1.Service, error) {
	var list corev1.ServiceList
	if err := t.kube.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	var matched []corev1.Service
	for i := range list.Items {
		svc := &list.Items[i]
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		if labelsContain(podLabels, svc.Spec.Selector) {
			matched = append(matched, *svc)
		}
	}
	return matched, nil
}

// labelsContain returns true if every key-value pair in required exists in labels.
func labelsContain(labels, required map[string]string) bool {
	for k, v := range required {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// podInEndpoints checks whether podIP appears in the ready addresses of the
// named Endpoints resource.
func (t *LabelBasedTrafficManager) podInEndpoints(
	ctx context.Context, epName, namespace, podIP string,
) (bool, error) {
	var ep corev1.Endpoints
	if err := t.kube.Get(ctx, types.NamespacedName{
		Name: epName, Namespace: namespace,
	}, &ep); err != nil {
		return false, fmt.Errorf("get endpoints %s: %w", epName, err)
	}

	for i := range ep.Subsets {
		for j := range ep.Subsets[i].Addresses {
			if ep.Subsets[i].Addresses[j].IP == podIP {
				return true, nil
			}
		}
	}
	return false, nil
}

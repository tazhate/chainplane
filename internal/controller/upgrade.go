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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// ---------------------------------------------------------------------------
// Upgrade annotations and constants
// ---------------------------------------------------------------------------

const (
	// annotationLastImage stores the last-applied image for change detection.
	annotationLastImage = "nodes.chainplane.io/last-image"
	// annotationLastClient stores the last-applied client for change detection.
	annotationLastClient = "nodes.chainplane.io/last-client"
	// annotationRestartedAt is patched on the pod template to trigger a rolling restart.
	annotationRestartedAt = "nodes.chainplane.io/restarted-at"
	// annotationPreviousImage preserves the pre-upgrade image for rollback.
	annotationPreviousImage = "nodes.chainplane.io/previous-image"

	// ConditionUpgrading is the condition type set while a rolling restart is
	// in progress.
	ConditionUpgrading = "Upgrading"

	// crashLoopRestartThreshold is the container restart count that triggers
	// an automatic rollback.
	crashLoopRestartThreshold = int32(3)
)

// ---------------------------------------------------------------------------
// Upgrade reconciler
// ---------------------------------------------------------------------------

// reconcileUpgrade detects image or client changes on the CR and drives a
// rolling restart of the StatefulSet. When the new image enters
// CrashLoopBackOff the previous image is restored automatically.
func (r *BlockchainNodeReconciler) reconcileUpgrade(ctx context.Context, node *nodesv1alpha1.BlockchainNode) error {
	sts := &appsv1.StatefulSet{}
	key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}
	if err := r.Get(ctx, key, sts); err != nil {
		return client.IgnoreNotFound(err)
	}

	wantImage := r.resolveImageForUpgrade(node)
	wantClient := node.Spec.Client

	stsAnn := sts.Annotations
	if stsAnn == nil {
		stsAnn = map[string]string{}
	}

	if stsAnn[annotationLastImage] == wantImage && stsAnn[annotationLastClient] == wantClient {
		return r.verifyRolloutHealth(ctx, node, sts)
	}

	return r.startRollingRestart(ctx, node, sts, wantImage, wantClient, stsAnn[annotationLastImage])
}

// startRollingRestart patches the StatefulSet to trigger a rolling pod restart
// and records the old image for a potential rollback.
func (r *BlockchainNodeReconciler) startRollingRestart(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	sts *appsv1.StatefulSet,
	newImage, newClient, previousImage string,
) error {
	logger := log.FromContext(ctx)
	logger.Info("image/client change detected, triggering rolling restart",
		"name", node.Name,
		"oldImage", previousImage,
		"newImage", newImage,
	)

	if err := r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionTrue, "RollingRestart", "Image or client changed, rolling restart in progress"); err != nil {
		return err
	}

	base := client.MergeFrom(sts.DeepCopy())
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}
	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = map[string]string{}
	}

	if previousImage != "" {
		sts.Annotations[annotationPreviousImage] = previousImage
	}
	sts.Annotations[annotationLastImage] = newImage
	sts.Annotations[annotationLastClient] = newClient
	sts.Spec.Template.Annotations[annotationRestartedAt] = time.Now().UTC().Format(time.RFC3339)

	if err := r.Patch(ctx, sts, base); err != nil {
		return fmt.Errorf("patching StatefulSet %s/%s for rolling restart: %w", sts.Namespace, sts.Name, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Rollout health
// ---------------------------------------------------------------------------

// verifyRolloutHealth checks whether a previously started rolling restart has
// completed successfully or needs a rollback.
func (r *BlockchainNodeReconciler) verifyRolloutHealth(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	sts *appsv1.StatefulSet,
) error {
	cond := findCondition(node.Status.Conditions, ConditionUpgrading)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return nil
	}

	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	if sts.Status.ReadyReplicas == replicas && sts.Status.UpdatedReplicas == replicas {
		if err := r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionFalse, "RolloutComplete", "All replicas are ready after upgrade"); err != nil {
			return err
		}
		return r.setCondition(ctx, node, nodesv1alpha1.ConditionReady, metav1.ConditionTrue, "Healthy", "Node is healthy after upgrade")
	}

	wantImage := r.resolveImageForUpgrade(node)
	if r.hasCrashLoop(ctx, node, wantImage) {
		return r.rollback(ctx, node, sts)
	}
	return nil
}

// ---------------------------------------------------------------------------
// CrashLoop detection
// ---------------------------------------------------------------------------

// hasCrashLoop returns true when any pod running the specified image has
// exceeded the restart threshold and is in CrashLoopBackOff.
func (r *BlockchainNodeReconciler) hasCrashLoop(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	targetImage string,
) bool {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods,
		client.InNamespace(node.Namespace),
		client.MatchingLabels(selectorLabels(node)),
	); err != nil {
		return false
	}

	for i := range pods.Items {
		for _, cs := range pods.Items[i].Status.ContainerStatuses {
			if cs.Name != mainContainerName {
				continue
			}
			if cs.RestartCount < crashLoopRestartThreshold {
				continue
			}
			if cs.State.Waiting == nil || cs.State.Waiting.Reason != "CrashLoopBackOff" {
				continue
			}
			if cs.Image == targetImage {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rollback
// ---------------------------------------------------------------------------

// rollback restores the StatefulSet to the previous image and marks the node
// as Degraded.
func (r *BlockchainNodeReconciler) rollback(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	sts *appsv1.StatefulSet,
) error {
	logger := log.FromContext(ctx)
	previousImage := sts.Annotations[annotationPreviousImage]

	if previousImage == "" {
		logger.Info("no previous image to roll back to, staying at current image", "name", node.Name)
	} else {
		logger.Info("rolling back to previous image", "name", node.Name, "image", previousImage)

		base := client.MergeFrom(sts.DeepCopy())
		if sts.Spec.Template.Annotations == nil {
			sts.Spec.Template.Annotations = map[string]string{}
		}
		for i, c := range sts.Spec.Template.Spec.Containers {
			if c.Name == mainContainerName {
				sts.Spec.Template.Spec.Containers[i].Image = previousImage
				break
			}
		}
		sts.Annotations[annotationLastImage] = previousImage
		sts.Spec.Template.Annotations[annotationRestartedAt] = time.Now().UTC().Format(time.RFC3339)

		if err := r.Patch(ctx, sts, base); err != nil {
			return fmt.Errorf("patching StatefulSet %s/%s for rollback: %w", sts.Namespace, sts.Name, err)
		}
	}

	if err := r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionFalse, "RollbackTriggered", "CrashLoopBackOff detected, rolled back to previous version"); err != nil {
		return err
	}

	base := client.MergeFrom(node.DeepCopy())
	node.Status.Phase = nodesv1alpha1.NodePhaseDegraded
	return r.Status().Patch(ctx, node, base)
}

// ---------------------------------------------------------------------------
// Condition writer (method form, persists to API)
// ---------------------------------------------------------------------------

// setCondition writes a condition to the node's status subresource via a
// server-side patch.
func (r *BlockchainNodeReconciler) setCondition(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	base := client.MergeFrom(node.DeepCopy())

	now := metav1.Now()
	cond := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: node.Generation,
	}

	found := false
	for i, c := range node.Status.Conditions {
		if c.Type != condType {
			continue
		}
		if c.Status != status {
			node.Status.Conditions[i] = cond
		} else {
			node.Status.Conditions[i].Reason = reason
			node.Status.Conditions[i].Message = message
			node.Status.Conditions[i].ObservedGeneration = node.Generation
		}
		found = true
		break
	}
	if !found {
		node.Status.Conditions = append(node.Status.Conditions, cond)
	}

	return r.Status().Patch(ctx, node, base)
}

// ---------------------------------------------------------------------------
// Image resolution for upgrade change detection
// ---------------------------------------------------------------------------

// resolveImageForUpgrade returns the canonical image identifier used for
// upgrade change detection. When the user provides an explicit image spec the
// result is repository:tag; otherwise it is a synthetic chain/client string.
func (r *BlockchainNodeReconciler) resolveImageForUpgrade(node *nodesv1alpha1.BlockchainNode) string {
	if node.Spec.Image != nil && node.Spec.Image.Repository != "" {
		return node.Spec.Image.Repository + ":" + node.Spec.Image.Tag
	}
	return fmt.Sprintf("%s/%s", node.Spec.Chain, node.Spec.Client)
}

package health

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// ReplacementPhase tracks the current stage of a pod replacement workflow.
type ReplacementPhase string

const (
	// ReplacementPending means replacement has been requested but not started.
	ReplacementPending ReplacementPhase = "Pending"
	// ReplacementDraining means traffic is being drained from the old pod.
	ReplacementDraining ReplacementPhase = "Draining"
	// ReplacementCreating means the replacement pod is being created.
	ReplacementCreating ReplacementPhase = "Creating"
	// ReplacementVerifying means the replacement pod is being health-checked.
	ReplacementVerifying ReplacementPhase = "Verifying"
	// ReplacementCompleting means traffic is switching to the new pod.
	ReplacementCompleting ReplacementPhase = "Completing"
	// ReplacementRollingBack means the replacement is being reverted.
	ReplacementRollingBack ReplacementPhase = "RollingBack"
)

// Annotation keys used to track replacement state on the BlockchainNode resource.
const (
	AnnotationReplacementPhase     = "nodes.chainplane.io/replacement-phase"
	AnnotationReplacementPod       = "nodes.chainplane.io/replacement-pod"
	AnnotationReplacementOldPod    = "nodes.chainplane.io/replacement-old-pod"
	AnnotationReplacementStartedAt = "nodes.chainplane.io/replacement-started-at"
)

// Label keys applied to pods managed by the replacement workflow.
const (
	LabelInstance       = "nodes.chainplane.io/instance"
	LabelReplacementFor = "nodes.chainplane.io/replacement-for"
	LabelReady          = "nodes.chainplane.io/ready"
)

// ReplacementConfig holds tunables for the replacement workflow.
type ReplacementConfig struct {
	// DrainTimeout is how long to wait for traffic to drain from the old pod.
	DrainTimeout time.Duration
	// SyncTimeout is how long to wait for the replacement pod to become healthy.
	SyncTimeout time.Duration
	// PollInterval is the interval between status checks during monitoring.
	PollInterval time.Duration
	// GracePeriodSeconds is the pod deletion grace period.
	GracePeriodSeconds int64
	// CleanupPVC controls whether the old pod's PVCs are deleted after replacement.
	CleanupPVC bool
}

// DefaultReplacementConfig returns production-ready defaults.
func DefaultReplacementConfig() ReplacementConfig {
	return ReplacementConfig{
		DrainTimeout:       30 * time.Second,
		SyncTimeout:        10 * time.Minute,
		PollInterval:       10 * time.Second,
		GracePeriodSeconds: 30,
		CleanupPVC:         false,
	}
}

// Drainer abstracts traffic drain and switch operations for testability.
type Drainer interface {
	// Drain removes the pod from service endpoints and waits for connections to close.
	Drain(ctx context.Context, podName, namespace string) error
	// SwitchTraffic atomically moves traffic from the old pod to the new pod.
	SwitchTraffic(ctx context.Context, oldPod, newPod, namespace string) error
	// ValidateTraffic checks that the new pod is receiving traffic.
	ValidateTraffic(ctx context.Context, podName, namespace string) (bool, error)
}

// TrafficManager is an alias kept for backward compatibility with callers
// that reference the interface by this name.
type TrafficManager = Drainer

// ReplacementManager orchestrates blue/green pod replacement for blockchain
// nodes. It creates a replacement pod, waits for it to sync, switches
// traffic, then removes the old pod.
type ReplacementManager struct {
	kube    client.Client
	checker *Checker
	drain   Drainer
	cfg     ReplacementConfig
}

// NewReplacementManager creates a ReplacementManager.
// If cfg is zero-value, DefaultReplacementConfig is used.
// drain may be nil; in that case drain/switch operations are skipped with a warning.
func NewReplacementManager(kube client.Client, checker *Checker, drain Drainer, cfg ReplacementConfig) *ReplacementManager {
	if cfg.SyncTimeout == 0 {
		cfg = DefaultReplacementConfig()
	}
	return &ReplacementManager{
		kube:    kube,
		checker: checker,
		drain:   drain,
		cfg:     cfg,
	}
}

// StartReplacement initiates the replacement workflow for a BlockchainNode.
// It sets the phase to Draining, cordons the old pod, drains traffic,
// creates a replacement pod, and advances to Verifying.
func (r *ReplacementManager) StartReplacement(ctx context.Context, node *nodesv1alpha1.BlockchainNode) error {
	log := slog.With("node", node.Name, "namespace", node.Namespace)
	log.Info("starting replacement workflow")

	oldPod := node.Name + "-0" // StatefulSet convention
	newPod := oldPod + "-replacement"

	if err := r.mergeAnnotations(ctx, node, map[string]string{
		AnnotationReplacementPhase:     string(ReplacementDraining),
		AnnotationReplacementOldPod:    oldPod,
		AnnotationReplacementPod:       newPod,
		AnnotationReplacementStartedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return &PhaseError{Phase: ReplacementDraining, Err: fmt.Errorf("set annotations: %w", err)}
	}

	// Cordon old pod.
	log.Info("cordoning old pod", "pod", oldPod)
	if err := r.labelPod(ctx, oldPod, node.Namespace, LabelReady, "false"); err != nil {
		if !apierrors.IsNotFound(err) {
			return &PhaseError{Phase: ReplacementDraining, Err: fmt.Errorf("cordon pod %s: %w", oldPod, err)}
		}
		log.Warn("old pod not found, skipping cordon", "pod", oldPod)
	}

	// Drain traffic.
	if r.drain != nil {
		drainCtx, cancel := context.WithTimeout(ctx, r.cfg.DrainTimeout)
		defer cancel()
		if err := r.drain.Drain(drainCtx, oldPod, node.Namespace); err != nil {
			log.Warn("drain returned error, continuing", "error", err)
		}
	} else {
		log.Warn("no drainer configured, skipping traffic drain")
	}

	// Create replacement pod.
	if err := r.mergeAnnotations(ctx, node, map[string]string{
		AnnotationReplacementPhase: string(ReplacementCreating),
	}); err != nil {
		return &PhaseError{Phase: ReplacementCreating, Err: fmt.Errorf("advance phase: %w", err)}
	}

	if err := r.spawnReplacementPod(ctx, node, oldPod, newPod); err != nil {
		return &PhaseError{Phase: ReplacementCreating, Err: err}
	}

	if err := r.mergeAnnotations(ctx, node, map[string]string{
		AnnotationReplacementPhase: string(ReplacementVerifying),
	}); err != nil {
		return &PhaseError{Phase: ReplacementVerifying, Err: fmt.Errorf("advance phase: %w", err)}
	}

	log.Info("replacement pod created, verifying health", "replacement", newPod)
	return nil
}

// MonitorReplacement checks replacement pod progress. Call on each
// reconciliation while the node is in Verifying phase. Returns true when
// the replacement pod is healthy and ready to receive traffic.
func (r *ReplacementManager) MonitorReplacement(ctx context.Context, node *nodesv1alpha1.BlockchainNode) (bool, error) {
	log := slog.With("node", node.Name, "namespace", node.Namespace)

	phase := nodeAnnotation(node, AnnotationReplacementPhase)
	if phase != string(ReplacementVerifying) {
		return false, &PhaseError{
			Phase: ReplacementPhase(phase),
			Err:   fmt.Errorf("expected %s, got %s", ReplacementVerifying, phase),
		}
	}

	newPod := nodeAnnotation(node, AnnotationReplacementPod)
	if newPod == "" {
		return false, fmt.Errorf("%w: %s", ErrReplacementAnnotationMissing, AnnotationReplacementPod)
	}

	// Fetch replacement pod.
	var pod corev1.Pod
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: newPod, Namespace: node.Namespace,
	}, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warn("replacement pod not found yet", "pod", newPod)
			return false, nil
		}
		return false, fmt.Errorf("get replacement pod %s: %w", newPod, err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		log.Info("replacement pod not running",
			"pod", newPod, "phase", pod.Status.Phase)
		return false, nil
	}

	if !containersReady(&pod) {
		log.Info("replacement pod containers not ready", "pod", newPod)
		return false, nil
	}

	// Evaluate health triggers on the replacement pod.
	chain := ChainToBlockchainType(node.Spec.Chain)
	status := r.checker.CheckAll(ctx, newPod, chain)
	if !status.Healthy() {
		fired := status.TriggeredNames()
		names := make([]string, len(fired))
		for i, n := range fired {
			names[i] = string(n)
		}
		log.Info("replacement pod has active triggers",
			"pod", newPod, "triggers", names)
		return false, nil
	}

	// Enforce sync timeout.
	if ts := nodeAnnotation(node, AnnotationReplacementStartedAt); ts != "" {
		if started, err := time.Parse(time.RFC3339, ts); err == nil {
			if time.Since(started) > r.cfg.SyncTimeout {
				return false, fmt.Errorf(
					"%w: pod %s not synced within %s",
					ErrReplacementTimeout, newPod, r.cfg.SyncTimeout,
				)
			}
		}
	}

	log.Info("replacement pod healthy and synced", "pod", newPod)
	return true, nil
}

// CompleteReplacement finalizes the replacement: switches traffic to the new
// pod, deletes the old pod (and optionally its PVCs), and clears annotations.
func (r *ReplacementManager) CompleteReplacement(ctx context.Context, node *nodesv1alpha1.BlockchainNode) error {
	log := slog.With("node", node.Name, "namespace", node.Namespace)

	if err := r.mergeAnnotations(ctx, node, map[string]string{
		AnnotationReplacementPhase: string(ReplacementCompleting),
	}); err != nil {
		return &PhaseError{Phase: ReplacementCompleting, Err: fmt.Errorf("advance phase: %w", err)}
	}

	oldPod := nodeAnnotation(node, AnnotationReplacementOldPod)
	newPod := nodeAnnotation(node, AnnotationReplacementPod)
	if oldPod == "" || newPod == "" {
		return fmt.Errorf("%w: old=%q new=%q", ErrReplacementAnnotationMissing, oldPod, newPod)
	}

	// Switch traffic.
	if r.drain != nil {
		log.Info("switching traffic", "old", oldPod, "new", newPod)
		if err := r.drain.SwitchTraffic(ctx, oldPod, newPod, node.Namespace); err != nil {
			return &PhaseError{Phase: ReplacementCompleting, Err: fmt.Errorf("switch traffic: %w", err)}
		}
		if ok, err := r.drain.ValidateTraffic(ctx, newPod, node.Namespace); err != nil {
			log.Warn("traffic validation error", "error", err)
		} else if !ok {
			log.Warn("traffic validation failed, proceeding")
		}
	} else {
		log.Warn("no drainer configured, labeling new pod ready")
		if err := r.labelPod(ctx, newPod, node.Namespace, LabelReady, "true"); err != nil {
			log.Warn("label new pod failed", "error", err)
		}
	}

	// Delete old pod.
	log.Info("deleting old pod", "pod", oldPod)
	if err := r.removePod(ctx, oldPod, node.Namespace); err != nil {
		return &PhaseError{Phase: ReplacementCompleting, Err: fmt.Errorf("delete old pod: %w", err)}
	}

	// Optional PVC cleanup.
	if r.cfg.CleanupPVC {
		if err := r.removePVC(ctx, oldPod, node.Namespace); err != nil {
			log.Warn("PVC cleanup failed", "error", err)
		}
	}

	if err := r.clearAnnotations(ctx, node); err != nil {
		return &PhaseError{Phase: ReplacementCompleting, Err: fmt.Errorf("clear annotations: %w", err)}
	}

	log.Info("replacement completed", "old", oldPod, "new", newPod)
	return nil
}

// RollbackReplacement aborts a replacement in progress. It deletes the
// replacement pod, restores the old pod to ready, and clears annotations.
func (r *ReplacementManager) RollbackReplacement(ctx context.Context, node *nodesv1alpha1.BlockchainNode) error {
	log := slog.With("node", node.Name, "namespace", node.Namespace)
	log.Warn("rolling back replacement")

	if err := r.mergeAnnotations(ctx, node, map[string]string{
		AnnotationReplacementPhase: string(ReplacementRollingBack),
	}); err != nil {
		return &PhaseError{Phase: ReplacementRollingBack, Err: fmt.Errorf("set rollback phase: %w", err)}
	}

	oldPod := nodeAnnotation(node, AnnotationReplacementOldPod)
	newPod := nodeAnnotation(node, AnnotationReplacementPod)

	// Delete replacement pod if it exists.
	if newPod != "" {
		var pod corev1.Pod
		if err := r.kube.Get(ctx, types.NamespacedName{
			Name: newPod, Namespace: node.Namespace,
		}, &pod); err == nil {
			log.Info("deleting replacement pod", "pod", newPod)
			if err := r.kube.Delete(ctx, &pod, client.GracePeriodSeconds(r.cfg.GracePeriodSeconds)); err != nil && !apierrors.IsNotFound(err) {
				log.Error("delete replacement pod failed", "pod", newPod, "error", err)
			}
		} else if !apierrors.IsNotFound(err) {
			log.Error("get replacement pod for rollback failed", "error", err)
		}
	}

	// Restore old pod.
	if oldPod != "" {
		log.Info("restoring old pod readiness", "pod", oldPod)
		if err := r.labelPod(ctx, oldPod, node.Namespace, LabelReady, "true"); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Warn("restore old pod label failed", "error", err)
			}
		}
	}

	if err := r.clearAnnotations(ctx, node); err != nil {
		return &PhaseError{Phase: ReplacementRollingBack, Err: fmt.Errorf("clear annotations: %w", err)}
	}

	log.Info("rollback completed", "old_pod", oldPod, "deleted_replacement", newPod)
	return nil
}

// IsReplacementInProgress checks whether a replacement workflow is active.
func IsReplacementInProgress(node *nodesv1alpha1.BlockchainNode) bool {
	phase := nodeAnnotation(node, AnnotationReplacementPhase)
	return phase != "" && phase != string(ReplacementCompleting)
}

// GetReplacementPhase returns the current replacement phase, or empty if none.
func GetReplacementPhase(node *nodesv1alpha1.BlockchainNode) ReplacementPhase {
	return ReplacementPhase(nodeAnnotation(node, AnnotationReplacementPhase))
}

// --- unexported helpers ---

// spawnReplacementPod creates a new pod cloned from the original with
// replacement labels.
func (r *ReplacementManager) spawnReplacementPod(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	oldName, newName string,
) error {
	var orig corev1.Pod
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: oldName, Namespace: node.Namespace,
	}, &orig); err != nil {
		return fmt.Errorf("get original pod %s: %w", oldName, err)
	}

	labels := cloneLabels(orig.Labels)
	labels[LabelInstance] = "replacement"
	labels[LabelReplacementFor] = oldName
	labels[LabelReady] = "false"
	labels["app.kubernetes.io/managed-by"] = "chainplane"
	labels["nodes.chainplane.io/chain"] = string(node.Spec.Chain)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newName,
			Namespace: node.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"nodes.chainplane.io/original-pod": oldName,
				"nodes.chainplane.io/created":      time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: *orig.Spec.DeepCopy(),
	}
	pod.Spec.NodeName = ""
	pod.Spec.Hostname = ""

	slog.Info("creating replacement pod", "pod", newName)
	if err := r.kube.Create(ctx, pod); err != nil {
		if apierrors.IsAlreadyExists(err) {
			slog.Info("replacement pod already exists, reusing", "pod", newName)
			return nil
		}
		return fmt.Errorf("create pod %s: %w", newName, err)
	}

	slog.Info("replacement pod created", "pod", newName, "uid", pod.UID)
	return nil
}

// removePod deletes a pod by name. Returns nil if already gone.
func (r *ReplacementManager) removePod(ctx context.Context, name, namespace string) error {
	var pod corev1.Pod
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: name, Namespace: namespace,
	}, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get pod %s for deletion: %w", name, err)
	}

	if err := r.kube.Delete(ctx, &pod, client.GracePeriodSeconds(r.cfg.GracePeriodSeconds)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete pod %s: %w", name, err)
	}
	slog.Info("pod deletion initiated", "pod", name)
	return nil
}

// removePVC deletes the StatefulSet PVC associated with the given pod.
func (r *ReplacementManager) removePVC(ctx context.Context, podName, namespace string) error {
	pvcName := "data-" + podName
	var pvc corev1.PersistentVolumeClaim
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: pvcName, Namespace: namespace,
	}, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			slog.Info("PVC not found, nothing to clean", "pvc", pvcName)
			return nil
		}
		return fmt.Errorf("get PVC %s: %w", pvcName, err)
	}

	slog.Info("deleting PVC", "pvc", pvcName)
	if err := r.kube.Delete(ctx, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete PVC %s: %w", pvcName, err)
	}
	return nil
}

// labelPod patches a single label on a pod via merge patch.
func (r *ReplacementManager) labelPod(ctx context.Context, name, namespace, key, value string) error {
	var pod corev1.Pod
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: name, Namespace: namespace,
	}, &pod); err != nil {
		return err
	}
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[key] = value
	return r.kube.Patch(ctx, &pod, patch)
}

// mergeAnnotations applies a set of annotations to the node resource via
// merge patch. It re-reads the resource first to avoid update conflicts.
func (r *ReplacementManager) mergeAnnotations(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	ann map[string]string,
) error {
	var current nodesv1alpha1.BlockchainNode
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: node.Name, Namespace: node.Namespace,
	}, &current); err != nil {
		return fmt.Errorf("re-read node: %w", err)
	}

	patch := client.MergeFrom(current.DeepCopy())
	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}
	for k, v := range ann {
		current.Annotations[k] = v
	}
	if err := r.kube.Patch(ctx, &current, patch); err != nil {
		return fmt.Errorf("patch annotations: %w", err)
	}
	node.Annotations = current.Annotations
	return nil
}

// clearAnnotations removes all replacement-related annotations from the node.
func (r *ReplacementManager) clearAnnotations(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
) error {
	var current nodesv1alpha1.BlockchainNode
	if err := r.kube.Get(ctx, types.NamespacedName{
		Name: node.Name, Namespace: node.Namespace,
	}, &current); err != nil {
		return fmt.Errorf("re-read node: %w", err)
	}

	patch := client.MergeFrom(current.DeepCopy())
	for _, key := range []string{
		AnnotationReplacementPhase,
		AnnotationReplacementPod,
		AnnotationReplacementOldPod,
		AnnotationReplacementStartedAt,
	} {
		delete(current.Annotations, key)
	}
	if err := r.kube.Patch(ctx, &current, patch); err != nil {
		return fmt.Errorf("clear annotations: %w", err)
	}
	node.Annotations = current.Annotations
	return nil
}

// containersReady returns true if every container in the pod reports ready.
func containersReady(pod *corev1.Pod) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false
	}
	for i := range pod.Status.ContainerStatuses {
		if !pod.Status.ContainerStatuses[i].Ready {
			return false
		}
	}
	return true
}

// nodeAnnotation safely reads an annotation from a BlockchainNode.
func nodeAnnotation(node *nodesv1alpha1.BlockchainNode, key string) string {
	if node.Annotations == nil {
		return ""
	}
	return node.Annotations[key]
}

// cloneLabels returns a shallow copy of the label map with extra capacity.
func cloneLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src)+4)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

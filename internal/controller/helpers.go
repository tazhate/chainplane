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
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/adapters"
)

// ---------------------------------------------------------------------------
// Block-lag thresholds
// ---------------------------------------------------------------------------

const (
	// lagThresholdETH is the maximum acceptable block lag for Ethereum nodes.
	lagThresholdETH = int64(30)
	// lagThresholdBTC is the maximum acceptable block lag for Bitcoin-family nodes.
	lagThresholdBTC = int64(2)
	// lagThresholdTRON is the maximum acceptable block lag for TRON nodes.
	lagThresholdTRON = int64(200)
)

// lagThreshold resolves the block-lag threshold for a node. An explicit
// spec.health.blockLagThreshold override takes precedence over the
// chain-specific default.
func lagThreshold(node *nodesv1alpha1.BlockchainNode) int64 {
	if node.Spec.Health.BlockLagThreshold != nil {
		return *node.Spec.Health.BlockLagThreshold
	}
	switch node.Spec.Chain {
	case nodesv1alpha1.ChainBitcoin, nodesv1alpha1.ChainLitecoin, nodesv1alpha1.ChainDash:
		return lagThresholdBTC
	case nodesv1alpha1.ChainTRON:
		return lagThresholdTRON
	default:
		return lagThresholdETH
	}
}

// ---------------------------------------------------------------------------
// Block time and stall threshold
// ---------------------------------------------------------------------------

// chainBlockTime returns the expected block production interval for a given
// chain. Unknown chains default to the Ethereum interval (12 s).
func chainBlockTime(chain nodesv1alpha1.Chain) time.Duration {
	switch chain {
	case nodesv1alpha1.ChainBitcoin:
		return 10 * time.Minute
	case nodesv1alpha1.ChainLitecoin:
		return 150 * time.Second
	case nodesv1alpha1.ChainDash:
		return 156 * time.Second
	case nodesv1alpha1.ChainEthereum, nodesv1alpha1.ChainEthereumArchive:
		return 12 * time.Second
	case nodesv1alpha1.ChainBSC:
		return 3 * time.Second
	case nodesv1alpha1.ChainTRON:
		return 3 * time.Second
	case nodesv1alpha1.ChainPolygon:
		return 2 * time.Second
	case nodesv1alpha1.ChainAvalanche:
		return 2 * time.Second
	case nodesv1alpha1.ChainXRP:
		return 4 * time.Second
	case nodesv1alpha1.ChainStellar:
		return 5 * time.Second
	case nodesv1alpha1.ChainTON:
		return 5 * time.Second
	case nodesv1alpha1.ChainNear:
		return 1 * time.Second
	case nodesv1alpha1.ChainSui:
		return 1 * time.Second
	case nodesv1alpha1.ChainCosmos:
		return 7 * time.Second
	case nodesv1alpha1.ChainAptos:
		return 1 * time.Second
	case nodesv1alpha1.ChainCardano:
		return 20 * time.Second
	default:
		return 12 * time.Second
	}
}

// syncStallThreshold computes how long a node's block height must remain
// frozen before it is considered stalled: 10 block times, but no less than
// five minutes for fast-finality chains.
func syncStallThreshold(chain nodesv1alpha1.Chain) time.Duration {
	const floor = 5 * time.Minute
	d := 10 * chainBlockTime(chain)
	if d < floor {
		return floor
	}
	return d
}

// ---------------------------------------------------------------------------
// Labels
// ---------------------------------------------------------------------------

// coreLabels returns the standard set of labels applied to every owned
// resource so that selectors and owner queries are consistent.
func coreLabels(node *nodesv1alpha1.BlockchainNode) map[string]string {
	return map[string]string{
		"app":   node.Name,
		"chain": string(node.Spec.Chain),
	}
}

// selectorLabels returns the minimal label set used in selector fields.
func selectorLabels(node *nodesv1alpha1.BlockchainNode) map[string]string {
	return map[string]string{"app": node.Name}
}

// ---------------------------------------------------------------------------
// Conditions
// ---------------------------------------------------------------------------

// setCondition upserts a metav1.Condition into node.Status.Conditions
// in-memory only (it does not persist). Callers must patch the status
// subresource afterwards. LastTransitionTime is only updated when the
// condition status actually flips.
func setCondition(node *nodesv1alpha1.BlockchainNode, condType string, status metav1.ConditionStatus, reason, message string, now metav1.Time) {
	for i, c := range node.Status.Conditions {
		if c.Type != condType {
			continue
		}
		if c.Status == status && c.Reason == reason && c.Message == message {
			return // no-op
		}
		transition := now
		if c.Status == status {
			transition = c.LastTransitionTime
		}
		node.Status.Conditions[i] = metav1.Condition{
			Type:               condType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: transition,
			ObservedGeneration: node.Generation,
		}
		return
	}
	node.Status.Conditions = append(node.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: node.Generation,
	})
}

// findCondition locates a condition by type. Returns nil when absent.
func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Image resolution
// ---------------------------------------------------------------------------

// resolveContainerImage determines the container image to use: the explicit
// spec.image override wins; otherwise the adapter supplies a default.
func resolveContainerImage(node *nodesv1alpha1.BlockchainNode, adapter adapters.ChainAdapter) string {
	if node.Spec.Image != nil && node.Spec.Image.Repository != "" {
		return node.Spec.Image.Repository + ":" + node.Spec.Image.Tag
	}
	return adapter.DefaultImage(node.Spec.Client)
}

// ---------------------------------------------------------------------------
// Container builders
// ---------------------------------------------------------------------------

// containerCommand returns the entrypoint override when the adapter
// implements ContainerCommandProvider, or nil to keep the image default.
func containerCommand(adapter adapters.ChainAdapter, node *nodesv1alpha1.BlockchainNode) []string {
	if cp, ok := adapter.(adapters.ContainerCommandProvider); ok {
		return cp.ContainerCommand(node.Spec)
	}
	return nil
}

// containerArgs merges adapter-supplied arguments with user-supplied ExtraArgs.
func containerArgs(adapter adapters.ChainAdapter, node *nodesv1alpha1.BlockchainNode) []string {
	var args []string
	if ap, ok := adapter.(adapters.ContainerArgsProvider); ok {
		args = append(args, ap.ContainerArgs(node.Spec)...)
	}
	args = append(args, node.Spec.ExtraArgs...)
	return args
}

// containerEnv assembles environment variables from the adapter, user extras,
// Bitcoin-family RPC credentials, and JVM flags.
func containerEnv(node *nodesv1alpha1.BlockchainNode) []corev1.EnvVar {
	adapter, _ := adapters.Get(node.Spec.Chain)
	var envs []corev1.EnvVar
	if ep, ok := adapter.(adapters.ContainerEnvProvider); ok {
		envs = append(envs, ep.ContainerEnv(node.Spec)...)
	}
	envs = append(envs, node.Spec.ExtraEnv...)

	envs = appendRPCCredentials(envs, node)
	envs = appendJVMFlags(envs, node)

	return envs
}

// appendRPCCredentials injects Secret-backed env vars for Bitcoin-family chains.
func appendRPCCredentials(envs []corev1.EnvVar, node *nodesv1alpha1.BlockchainNode) []corev1.EnvVar {
	type credPair struct{ userKey, passKey string }
	mapping := map[nodesv1alpha1.Chain]credPair{
		nodesv1alpha1.ChainBitcoin:  {"BTC_RPC_USER", "BTC_RPC_PASSWORD"},
		nodesv1alpha1.ChainDash:     {"DASH_RPC_USER", "DASH_RPC_PASSWORD"},
		nodesv1alpha1.ChainLitecoin: {"LTC_RPC_USER", "LTC_RPC_PASSWORD"},
	}
	cp, ok := mapping[node.Spec.Chain]
	if !ok {
		return envs
	}

	secretName := node.Name + "-rpc-credentials"
	optional := true
	return append(envs,
		corev1.EnvVar{
			Name: cp.userKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "rpc-user",
					Optional:             &optional,
				},
			},
		},
		corev1.EnvVar{
			Name: cp.passKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "rpc-password",
					Optional:             &optional,
				},
			},
		},
	)
}

// appendJVMFlags injects JAVA_OPTS from the jvm-flags annotation (used by
// java-tron and other JVM-based chains).
func appendJVMFlags(envs []corev1.EnvVar, node *nodesv1alpha1.BlockchainNode) []corev1.EnvVar {
	if flags, ok := node.Annotations["nodes.chainplane.io/jvm-flags"]; ok && flags != "" {
		return append(envs, corev1.EnvVar{Name: "JAVA_OPTS", Value: flags})
	}
	return envs
}

// ---------------------------------------------------------------------------
// Annotation helpers
// ---------------------------------------------------------------------------

// persistAnnotations writes metadata.annotations to the API server while
// preserving in-memory status modifications. r.Update() writes the full
// object and the API response overwrites node.Status, so we save/restore it.
func (r *BlockchainNodeReconciler) persistAnnotations(ctx context.Context, node *nodesv1alpha1.BlockchainNode) error {
	saved := node.Status.DeepCopy()
	if err := r.Update(ctx, node); err != nil {
		return fmt.Errorf("persisting annotations for %s/%s: %w", node.Namespace, node.Name, err)
	}
	node.Status = *saved
	return nil
}

// ---------------------------------------------------------------------------
// Duration formatting
// ---------------------------------------------------------------------------

// humanDuration formats a duration to a practical granularity: seconds for
// short durations, minutes for medium, and quarter-hours for long ones.
func humanDuration(d time.Duration) string {
	switch {
	case d < 2*time.Minute:
		return d.Round(time.Second).String()
	case d < 2*time.Hour:
		return d.Round(time.Minute).String()
	default:
		return d.Round(15 * time.Minute).String()
	}
}

// ---------------------------------------------------------------------------
// Hex parser
// ---------------------------------------------------------------------------

// hexToInt64 converts a 0x-prefixed hex string to int64. Returns 0 for
// empty or malformed input.
func hexToInt64(s string) int64 {
	if len(s) > 2 && s[:2] == "0x" {
		s = s[2:]
	}
	if s == "" {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(s, "%x", &n)
	return n
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

// tipHTTPClient returns an *http.Client with a 10-second timeout suitable
// for public chain-tip API calls.
func tipHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// ---------------------------------------------------------------------------

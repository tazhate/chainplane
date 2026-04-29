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
// Package adapters implements the ChainAdapter interface for each supported blockchain.
package adapters

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// SyncStatus holds the current sync state returned by a HealthCheck.
type SyncStatus struct {
	IsSyncing    bool
	CurrentBlock int64
	HighestBlock int64
	Peers        int32
	Progress     float64 // 0.0-100.0
	// StallExempt suppresses the height-freeze stall detector for this cycle.
	// Set by adapters that have internal progress not reflected in CurrentBlock
	// (e.g. Reth pipeline stages, Stellar "Joining SCP").
	StallExempt bool
}

// ChainAdapter defines the contract that each blockchain implementation must satisfy.
type ChainAdapter interface {
	// DefaultImage returns the default container image for this chain/client.
	DefaultImage(client string) string
	// ConfigTemplate renders the chain-specific config file content.
	ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (filename string, content string, err error)
	// HealthCheck calls the chain RPC and returns the current sync status.
	HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error)
	// LivenessProbe returns a Kubernetes liveness probe for this chain.
	LivenessProbe(spec chainsv1alpha2.ChainInstanceSpec) *corev1.Probe
	// NodeSelector returns the label selector for the hardware tier.
	NodeSelector(nodeGroup chainsv1alpha2.NodeGroup) map[string]string
	// ContainerPorts returns the container ports to expose.
	ContainerPorts(spec chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort
}

// ContainerArgsProvider is an optional interface that adapters can implement
// to inject chain-specific command-line arguments into the main container.
// These args are prepended before any user-supplied ExtraArgs.
type ContainerArgsProvider interface {
	ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string
}

// ContainerCommandProvider is an optional interface that adapters can implement
// to override the container entrypoint (command). When not implemented, the
// image's default ENTRYPOINT is used.
type ContainerCommandProvider interface {
	ContainerCommand(spec chainsv1alpha2.ChainInstanceSpec) []string
}

// ContainerEnvProvider is an optional interface that adapters can implement
// to inject chain-specific environment variables. Variables are merged with
// node.Spec.ExtraEnv (adapter vars first, spec vars may override).
type ContainerEnvProvider interface {
	ContainerEnv(spec chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar
}

// NodePortProvider is an optional interface that adapters can implement
// to request a NodePort service instead of ClusterIP. The returned map
// maps container port numbers to the desired NodePort values (0 = auto-assign).
// This is required for chains that use UDP-based P2P protocols (e.g. TON ADNL)
// where external peers must reach the node directly via the host IP.
// The node name is passed so adapters can assign unique ports per instance.
type NodePortProvider interface {
	NodePorts(spec chainsv1alpha2.ChainInstanceSpec, name string) map[int32]int32
}

// StartupProbeProvider is an optional interface for adapters with long startup
// times. When implemented, the returned probe is used as the container's
// startupProbe -- it runs before the liveness probe and gives the node time to
// complete expensive init steps (e.g. TRON LiteFullNode loadTransForLiteNode).
// Once the startup probe succeeds, normal liveness probing begins.
type StartupProbeProvider interface {
	StartupProbe(spec chainsv1alpha2.ChainInstanceSpec) *corev1.Probe
}

// InitContainerProvider is an optional interface that adapters can implement
// to add chain-specific init containers (e.g. SUI formal snapshot download).
// These run AFTER the snapshot-restore init container.
type InitContainerProvider interface {
	InitContainers(spec chainsv1alpha2.ChainInstanceSpec) []corev1.Container
}

// SidecarProvider is an optional interface that adapters can implement
// to inject long-running sidecar containers alongside the main node container.
// Common use case: metric exporters for chains without native Prometheus support.
// These sidecars are prepended before user-supplied spec.Sidecars.
type SidecarProvider interface {
	Sidecars(spec chainsv1alpha2.ChainInstanceSpec) []corev1.Container
}

// VersionProvider is an optional interface that adapters can implement to
// enable automatic version tracking via ChainVersionCatalog.
type VersionProvider interface {
	VersionPolicy() ChainVersionPolicy
}

// ResourceDefaults holds recommended resource allocations for a blockchain node.
// Values are based on official documentation and real-world operational experience.
type ResourceDefaults struct {
	// CPURequest is the recommended CPU request (e.g. "4").
	CPURequest resource.Quantity
	// MemoryRequest is the recommended memory request (e.g. "8Gi").
	MemoryRequest resource.Quantity
	// Storage is the recommended PVC size (e.g. "500Gi").
	Storage resource.Quantity
}

// DefaultResourcesProvider is an optional interface that adapters can implement
// to declare recommended resource allocations. Used as defaults when the user
// does not specify Resources in ChainInstanceSpec.
type DefaultResourcesProvider interface {
	DefaultResources() ResourceDefaults
}

// ChainVersionPolicy describes how to find the latest version of a chain image.
type ChainVersionPolicy struct {
	// Registry is "docker.io" or "ghcr.io".
	Registry string
	// Repository is the image repository, e.g. "lncm/bitcoind".
	Repository string
	// TagPattern is a regexp that matching tags must satisfy, e.g. `^v\d+`.
	TagPattern string
	// TagPrefix is stripped from the tag before semver comparison, e.g. "GreatVoyage-" for TRON.
	TagPrefix string
}

// registry holds all registered adapters behind a RWMutex for thread safety.
var (
	registryMu sync.RWMutex
	registry   = make(map[chainsv1alpha2.Chain]ChainAdapter)
)

// Register adds a ChainAdapter implementation for the given chain.
// Safe for concurrent use; typically called from init().
func Register(chain chainsv1alpha2.Chain, adapter ChainAdapter) {
	registryMu.Lock()
	registry[chain] = adapter
	registryMu.Unlock()
}

// Get returns the ChainAdapter for the given chain, or (nil, false) if not found.
func Get(chain chainsv1alpha2.Chain) (ChainAdapter, bool) {
	registryMu.RLock()
	a, ok := registry[chain]
	registryMu.RUnlock()
	return a, ok
}

// MustGet returns the ChainAdapter for the given chain.
// It panics if no adapter is registered -- intended for init-time wiring only.
func MustGet(chain chainsv1alpha2.Chain) ChainAdapter {
	a, ok := Get(chain)
	if !ok {
		panic(fmt.Sprintf("adapters: no adapter registered for chain %q", chain))
	}
	return a
}

// All returns a snapshot of all registered adapters, keyed by chain.
func All() map[chainsv1alpha2.Chain]ChainAdapter {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make(map[chainsv1alpha2.Chain]ChainAdapter, len(registry))
	for k, v := range registry {
		out[k] = v
	}
	return out
}

// DefaultNodeSelector returns a node selector for specialised node groups only.
// Generic groups (light/medium/heavy) return nil so pods schedule on any node.
func DefaultNodeSelector(nodeGroup chainsv1alpha2.NodeGroup) map[string]string {
	switch nodeGroup {
	case chainsv1alpha2.NodeGroupStorage:
		return map[string]string{"workload-type": "storage"}
	case chainsv1alpha2.NodeGroupBlockchain:
		return map[string]string{"node-role.kubernetes.io/blockchain": "true"}
	default:
		return nil
	}
}

// DefaultLivenessProbe returns a generic TCP liveness probe on the given port.
func DefaultLivenessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       30,
		TimeoutSeconds:      10,
		FailureThreshold:    3,
	}
}

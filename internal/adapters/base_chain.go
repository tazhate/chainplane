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
package adapters

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const (
	baseSequencerURL = "https://mainnet-sequencer.base.org"
	// baseConsensusDataDir holds base-consensus' P2P key and bootstore.
	baseConsensusDataDir = "/data/base-consensus"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

// baseChainAdapter runs Base the way base/node does: base-reth-node as the
// main container and base-consensus as a sidecar, both from the ghcr.io/base/node
// image. Base left the superchain registry, so op-reth and op-node have no
// built-in Base config; both Base binaries ship it. Ports, the reth flags and
// the engine API wiring are shared with opRethProtocolAdapter.
type baseChainAdapter struct {
	opRethProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainBase, &baseChainAdapter{
		opRethProtocolAdapter: newOpRethProtocolAdapter(chainsv1alpha2.ChainBase, "base", "", baseSequencerURL),
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

// ContainerCommand selects base-reth-node; the image itself starts supervisord.
func (a *baseChainAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"/app/base-reth-node"}
}

// ContainerArgs runs base-reth-node on its built-in Base chain spec. It has
// no --rollup.disable-tx-pool-gossip flag.
func (a *baseChainAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return rethNodeArgs(a.rethChain, a.sequencerURL, spec)
}

// Sidecars adds base-consensus, Base's consensus client, which has the Base
// rollup config built in (chain 8453). L1 endpoints come from L1_RPC_URL and
// L1_BEACON_URL in spec.extraEnv, and BASE_NODE_* entries there pass through.
func (a *baseChainAdapter) Sidecars(spec chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return []corev1.Container{{
		Name:    "base-consensus",
		Image:   a.mainImage(spec),
		Command: waitForEngineSecret("/app/base-consensus", baseConsensusDataDir),
		Args: []string{
			"node",
			"--chain", "8453",
			"--l2-engine-rpc", "http://127.0.0.1:" + strconv.Itoa(opRethAuthPort),
			"--l2-engine-jwt-secret", opRethJWTPath,
			"--rpc.addr", "0.0.0.0", "--port", strconv.Itoa(opNodeRPCPort),
			"--metrics.enabled", "--metrics.addr", "0.0.0.0", "--metrics.port", strconv.Itoa(opNodeMetricsPort),
			"--p2p.listen.tcp", strconv.Itoa(opNodeP2PPort), "--p2p.listen.udp", strconv.Itoa(opNodeP2PPort),
			"--p2p.priv.path", baseConsensusDataDir + "/p2p_priv.txt",
			"--p2p.bootstore", baseConsensusDataDir + "/bootstore",
		},
		Env: l1Env(spec.ExtraEnv, "BASE_NODE_L1_ETH_RPC", "BASE_NODE_L1_BEACON", "BASE_NODE_"),
		Ports: []corev1.ContainerPort{
			{Name: "cl-rpc", ContainerPort: opNodeRPCPort, Protocol: corev1.ProtocolTCP},
			{Name: "cl-p2p-tcp", ContainerPort: opNodeP2PPort, Protocol: corev1.ProtocolTCP},
			{Name: "cl-p2p-udp", ContainerPort: opNodeP2PPort, Protocol: corev1.ProtocolUDP},
			{Name: "cl-metrics", ContainerPort: opNodeMetricsPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}}
}

// mainImage is the image of the main container, which the sidecar shares so
// both binaries always come from the same release.
func (a *baseChainAdapter) mainImage(spec chainsv1alpha2.ChainInstanceSpec) string {
	if spec.Image != nil && spec.Image.Repository != "" {
		return spec.Image.Repository + ":" + spec.Image.Tag
	}
	return a.DefaultImage(spec.Client)
}

func (a *baseChainAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

// VersionPolicy tracks ghcr.io/base/node, which carries both binaries.
func (a *baseChainAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "base/node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// ClientVersionPolicies is empty: the sidecar runs the main image.
func (a *baseChainAdapter) ClientVersionPolicies() map[string]ChainVersionPolicy {
	return nil
}

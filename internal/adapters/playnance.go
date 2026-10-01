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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// playnance is PlayBlock (chain id 1829), a Gelato-hosted AnyTrust Orbit
// chain whose parent chain is Arbitrum Nova, so L1_RPC_URL defaults to the
// public Nova RPC and no beacon endpoint is needed. The chain info and the
// DAS REST aggregator are only published in the Gelato dashboard and are not
// built into Nitro: pass them with spec.extraArgs (--chain.info-json=...,
// --node.da.anytrust.enable, --node.da.anytrust.rest-aggregator.enable,
// --node.da.anytrust.rest-aggregator.urls=...); without the chain info
// Nitro stops at startup.
var playnance = nitroChain{
	ChainID:          1829,
	ForwardingTarget: "https://rpc.playblock.io",
	ParentChainURL:   "https://nova.arbitrum.io/rpc",
	HTTPPort:         8547,
	WSPort:           8548,
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type playnanceAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainPlaynance, &playnanceAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *playnanceAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainPlaynance, client)
}

// ConfigTemplate renders the Nitro config. The chain info and DAS endpoints
// must come from spec.extraArgs, see playnance.
func (a *playnanceAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := nitroConfig(playnance)
	return nitroConfigFile, cfg, err
}

func (a *playnanceAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *playnanceAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return nitroPorts(playnance)
}

func (a *playnanceAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return nitroArgs(playnance)
}

// ContainerEnv injects the Arbitrum Nova endpoint; override L1_RPC_URL via
// extraEnv.
func (a *playnanceAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return nitroEnv(playnance)
}

func (a *playnanceAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *playnanceAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		// Releases are tagged v<version>-<short commit>; plain v<version>
		// tags are no longer published.
		TagPattern: `^(?P<version>v\d+\.\d+\.\d+)-[0-9a-f]{7}$`,
	}
}

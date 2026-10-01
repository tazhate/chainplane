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

const defaultArbitrumL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type arbitrumAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainArbitrum, &arbitrumAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// arbitrumChain returns Arbitrum One, or Arbitrum Sepolia for testnet. Both
// are built into Nitro, which takes the sequencer feed and forwarding target
// from its own chain info.
func arbitrumChain(spec chainsv1alpha2.ChainInstanceSpec) nitroChain {
	c := nitroChain{
		ChainID:         42161,
		ParentChainURL:  defaultArbitrumL1URL,
		BlobsFromBeacon: true,
		HTTPPort:        8545,
		WSPort:          8546,
	}
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		c.ChainID = 421614
	}
	return c
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *arbitrumAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainArbitrum, client)
}

func (a *arbitrumAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := nitroConfig(arbitrumChain(spec))
	return nitroConfigFile, cfg, err
}

func (a *arbitrumAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *arbitrumAdapter) ContainerPorts(spec chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return nitroPorts(arbitrumChain(spec))
}

// ContainerArgs points Nitro at the rendered config and the L1 execution and
// beacon endpoints. L1_RPC_URL and L1_BEACON_URL are set by ContainerEnv and
// can be overridden via extraEnv.
func (a *arbitrumAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return nitroArgs(arbitrumChain(spec))
}

// ContainerEnv injects the L1 endpoints required by Arbitrum Nitro.
func (a *arbitrumAdapter) ContainerEnv(spec chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return nitroEnv(arbitrumChain(spec))
}

func (a *arbitrumAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

func (a *arbitrumAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		// Releases are tagged v<version>-<short commit>; plain v<version>
		// tags are no longer published.
		TagPattern: `^(?P<version>v\d+\.\d+\.\d+)-[0-9a-f]{7}$`,
	}
}

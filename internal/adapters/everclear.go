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

// everclear is the Everclear hub (chain id 25327), a Gelato-hosted Orbit
// chain on Ethereum. Its chain info is not published outside the Gelato
// dashboard and is not built into Nitro, so it has to be passed with
// spec.extraArgs (--chain.info-json=...); without it Nitro stops at
// startup. Everclear wound down in May 2026 and the chain stopped producing
// blocks, so a node can only serve the frozen history.
var everclear = nitroChain{
	ChainID:         25327,
	ParentChainURL:  "http://ethereum:8545",
	BlobsFromBeacon: true,
	HTTPPort:        8547,
	WSPort:          8548,
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type everclearAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainEverclear, &everclearAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *everclearAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainEverclear, client)
}

// ConfigTemplate renders the Nitro config. The chain info (and the AnyTrust
// DAS endpoints) must come from spec.extraArgs, see everclear.
func (a *everclearAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := nitroConfig(everclear)
	return nitroConfigFile, cfg, err
}

func (a *everclearAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *everclearAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return nitroPorts(everclear)
}

func (a *everclearAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return nitroArgs(everclear)
}

// ContainerEnv injects the Ethereum execution and beacon endpoints; override
// L1_RPC_URL and L1_BEACON_URL via extraEnv.
func (a *everclearAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return nitroEnv(everclear)
}

func (a *everclearAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *everclearAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		// Releases are tagged v<version>-<short commit>; plain v<version>
		// tags are no longer published.
		TagPattern: `^(?P<version>v\d+\.\d+\.\d+)-[0-9a-f]{7}$`,
	}
}

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

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type coreAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainCore, &coreAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *coreAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainCore, client)
}

func (a *coreAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", coreConfig, nil
}

func (a *coreAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *coreAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

// ContainerCommand runs geth directly. The image entrypoint reads
// /core/config/config.toml and genesis.json, which the image does not ship;
// core-chain's geth has the Core genesis built in (--mainnet, chain id 1116;
// --pigeon for the test2 network) and initialises an empty datadir from it.
func (a *coreAdapter) ContainerCommand(spec chainsv1alpha2.ChainInstanceSpec) []string {
	network := "--mainnet"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		network = "--pigeon"
	}
	return []string{"geth", network, "--config", "/config/config.toml"}
}

func (a *coreAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return gethMetricsArgs(6060)
}

func (a *coreAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *coreAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "coredao-org/core-chain",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const coreConfig = `# Core Chain (CoreDAO) EVM node configuration
[Eth]
SyncMode = "snap"

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool", "debug"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

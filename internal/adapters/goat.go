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

const defaultGoatL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type goatAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainGoat, &goatAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *goatAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainGoat, client)
}

func (a *goatAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", goatConfig, nil
}

func (a *goatAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *goatAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *goatAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *goatAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *goatAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "goatnetwork/goat-geth",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// ContainerEnv injects the L1_RPC_URL required by the Goat Bitcoin L2 node.
// NOTE: GOAT Network is a Bitcoin L2 with Ethereum settlement. L1_RPC_URL here points to Ethereum for state derivation.
func (a *goatAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultGoatL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const goatConfig = `# Goat Network (Bitcoin L2 with EVM) node configuration
[Node]
DataDir = "/data"

[Node.HTTP]
Host = "0.0.0.0"
Port = 8545
VirtualHosts = ["*"]
Modules = ["eth", "net", "web3", "debug"]

[Node.WS]
Host = "0.0.0.0"
Port = 8546
Origins = ["*"]
Modules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

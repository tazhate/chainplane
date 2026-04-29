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

const defaultOptimismL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type optimismAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainOptimism, &optimismAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *optimismAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainOptimism, client)
}

func (a *optimismAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", optimismConfig, nil
}

func (a *optimismAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *optimismAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerArgs passes the config file path to op-geth.
func (a *optimismAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--config", "/config/config.toml",
		"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060",
	}
}

// ContainerEnv injects the OP_NODE_L1_ETH_RPC environment variable required by op-geth.
func (a *optimismAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "OP_NODE_L1_ETH_RPC", Value: defaultOptimismL1URL},
	}
}

func (a *optimismAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const optimismConfig = `# op-geth configuration for OP Mainnet
[Eth]
NetworkId = 10
SyncMode = "snap"

[Node]
DataDir = "/data"

[Node.HTTPHost]
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPVirtualHosts = ["*"]
HTTPCorsDomain = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool"]

[Node.WSHost]
WSHost = "0.0.0.0"
WSPort = 8546
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

func (a *optimismAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "us-docker.pkg.dev",
		Repository: "oplabs-tools-artifacts/images/op-geth",
		TagPattern: `^v\d+\.\d+`,
	}
}

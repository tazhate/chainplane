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

// NOTE: opBNB uses bnb-chain/op-geth fork (BNB-specific patches). Upstream oplabs/op-geth may lack BNB-specific consensus modifications.

// defaultOpBNBL1URL points to the BNB Smart Chain L1 node (not Ethereum).
const defaultOpBNBL1URL = "http://bsc:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type opbnbAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainOpBNB, &opbnbAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *opbnbAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainOpBNB, client)
}

func (a *opbnbAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", opbnbConfig, nil
}

func (a *opbnbAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *opbnbAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

// ContainerArgs passes the mounted config to bnb-chain op-geth and selects
// opBNB mainnet with the fork's own --opBNBMainnet flag, which loads the
// bundled genesis; it has no --op-network entry for opBNB.
func (a *opbnbAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return append(opGethArgs(), "--opBNBMainnet")
}

func (a *opbnbAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *opbnbAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "bnb-chain/op-geth",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// ContainerEnv injects L1_RPC_URL pointing to the BNB Chain L1 (BSC), not Ethereum.
func (a *opbnbAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultOpBNBL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// HTTP and WS settings are plain [Node] fields in geth's node.Config; geth
// rejects sub-tables or unknown keys such as HTTPCorsDomain at startup.
const opbnbConfig = `# op-geth configuration for opBNB Mainnet (BNB Chain L2, OP Stack)
[Eth]
NetworkId = 204
SyncMode = "snap"

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool"]
WSHost = "0.0.0.0"
WSPort = 8546
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

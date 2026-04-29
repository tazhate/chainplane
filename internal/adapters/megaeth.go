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

// NOTE: MegaETH does not have a public node image as of 2026. This adapter is a placeholder.

const defaultMegaETHL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type megaethAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMegaETH, &megaethAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *megaethAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMegaETH, client)
}

func (a *megaethAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", megaethConfig, nil
}

func (a *megaethAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *megaethAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *megaethAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *megaethAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

// ContainerEnv injects the L1_RPC_URL environment variable required by MegaETH (L2).
func (a *megaethAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultMegaETHL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const megaethConfig = `# MegaETH node configuration
[Eth]
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

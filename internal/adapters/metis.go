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

const defaultMetisL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type metisAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMetis, &metisAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *metisAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMetis, client)
}

func (a *metisAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", metisConfig, nil
}

func (a *metisAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const metisConfig = `# Metis Andromeda l2geth configuration
[Eth]
NetworkId = 1088
SyncMode = "snap"

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
`

func (a *metisAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *metisAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "metisprotocol/l2geth",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *metisAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerArgs enables Prometheus metrics endpoint on Metis l2geth.
func (a *metisAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

// ContainerEnv injects the L1_RPC_URL environment variable required by l2geth.
func (a *metisAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultMetisL1URL},
	}
}

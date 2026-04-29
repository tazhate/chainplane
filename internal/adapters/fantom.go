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

type fantomAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainFantom, &fantomAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *fantomAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainFantom, client)
}

func (a *fantomAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", fantomConfig, nil
}

func (a *fantomAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *fantomAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *fantomAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(5050), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *fantomAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--datadir", "/data",
		"--http",
		"--http.addr", "0.0.0.0",
		"--http.port", "8545",
		"--http.api", "eth,net,web3,ftm",
		"--http.vhosts", "*",
		"--http.corsdomain", "*",
		"--ws",
		"--ws.addr", "0.0.0.0",
		"--ws.port", "8546",
		"--ws.api", "eth,net,web3",
		"--ws.origins", "*",
		"--port", "5050",
		"--maxpeers", "50",
		"--cache", "4096",
		"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060",
	}
}

func (a *fantomAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

func (a *fantomAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "fantomfoundation/go-opera",
		TagPattern: `^v\d+\.\d+\.\d+`,
	}
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const fantomConfig = `# Fantom Opera mainnet RPC node
[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "ftm"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":5050"
`

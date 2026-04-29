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

type wemixAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainWemix, &wemixAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8588},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *wemixAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainWemix, client)
}

func (a *wemixAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", wemixConfig, nil
}

func (a *wemixAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *wemixAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "http", ContainerPort: 8588, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8598, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 8589, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *wemixAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *wemixAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *wemixAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "wemixnetwork/wemix",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const wemixConfig = `# Wemix EVM-compatible chain node configuration
[Eth]
SyncMode = "snap"
NetworkId = 1111

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8588
HTTPVirtualHosts = ["*"]
HTTPCorsDomain = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool"]
WSHost = "0.0.0.0"
WSPort = 8598
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":8589"
`

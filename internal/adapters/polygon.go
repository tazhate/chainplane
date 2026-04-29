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

type polygonAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainPolygon, &polygonAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string)
// --------------------------------------------------------------------------

const polygonConfig = `# Bor 2.x mainnet RPC node with Heimdall sidecar
chain = "mainnet"
datadir = "/data"
syncmode = "full"
gcmode = "archive"

[jsonrpc]
  [jsonrpc.http]
    enabled = true
    host = "0.0.0.0"
    port = 8545
    api = ["eth", "net", "web3", "txpool", "bor"]
    vhosts = ["*"]
    corsdomain = ["*"]
  [jsonrpc.ws]
    enabled = true
    host = "0.0.0.0"
    port = 8546
    api = ["eth", "net", "web3"]
    origins = ["*"]

[p2p]
  maxpeers = 50

[txpool]
  pricelimit = 25000000000

[bor]
  withoutheimdall = false

[heimdall]
  url = "http://localhost:1317"

[log]
  verbosity = 3
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *polygonAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainPolygon, client)
}

func (a *polygonAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", polygonConfig, nil
}

// ContainerCommand overrides entrypoint to use Bor 2.x server subcommand.
func (a *polygonAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"bor", "server", "--config", "/config/config.toml"}
}

func (a *polygonAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *polygonAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *polygonAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30303, HostPort: 30303, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30303, HostPort: 30303, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *polygonAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *polygonAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *polygonAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "0xpolygon/bor",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

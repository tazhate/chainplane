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

// NOTE: fantomfoundation/sonic:v1.2.0 was the legacy Fantom Opera client and is
// explicitly INCOMPATIBLE with Sonic mainnet. Sonic is a standalone L1 chain.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type sonicAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainSonic, &sonicAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 18545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *sonicAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainSonic, client)
}

func (a *sonicAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", sonicConfig, nil
}

func (a *sonicAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *sonicAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 18545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 18546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 5050, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 5050, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *sonicAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *sonicAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const sonicConfig = `# Sonic (Fantom successor) mainnet RPC node
[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 18545
HTTPModules = ["eth", "net", "web3", "ftm", "sonic"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 18546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":5050"
`

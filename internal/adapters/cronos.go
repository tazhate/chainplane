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

type cronosAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainCronos, &cronosAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *cronosAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainCronos, client)
}

func (a *cronosAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", cronosConfig, nil
}

func (a *cronosAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *cronosAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 26656, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

func (a *cronosAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

func (a *cronosAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "crypto-org-chain/cronos",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const cronosConfig = `# Cronos mainnet RPC node
[json-rpc]
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
api = "eth,net,web3,txpool"
enable = true

[telemetry]
enabled = true
prometheus-retention-time = 60
prometheus-addr = "0.0.0.0:26660"

[p2p]
laddr = "tcp://0.0.0.0:26656"
max_num_inbound_peers = 40
max_num_outbound_peers = 10

[statesync]
enable = false
`

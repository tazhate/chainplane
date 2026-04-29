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

type haqqAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainHaqq, &haqqAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *haqqAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainHaqq, client)
}

func (a *haqqAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", haqqConfig, nil
}

func (a *haqqAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *haqqAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(26656), corev1.ContainerPort{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP})
}

func (a *haqqAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "26660"}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const haqqConfig = `# Haqq Network (Islamic Coin) — Cosmos EVM chain node configuration
[json-rpc]
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
api = "eth,net,web3,txpool"
enable = true

[p2p]
laddr = "tcp://0.0.0.0:26656"
max_num_inbound_peers = 40
max_num_outbound_peers = 10

[statesync]
enable = false
`

func (a *haqqAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *haqqAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "alhaqq/haqq",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

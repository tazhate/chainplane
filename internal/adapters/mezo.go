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

// Mezo is Cosmos SDK with EVMOS EVM extension (not OP Stack).

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type mezoAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMezo, &mezoAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *mezoAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMezo, client)
}

func (a *mezoAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", mezoConfig, nil
}

// mezoNode bootstraps mezo_31612-1. The image ENTRYPOINT is a validator
// setup script (needs a keyring mnemonic, rewrites the configs on every
// start), and the image has no download tools, so the script runs mezod
// directly on the busybox toolbox. Mezo serves no snapshots itself;
// Lavender.Five advertises state sync and is the only public RPC.
var mezoNode = cosmosNode{
	Binary:        "mezod",
	ChainID:       "mezo_31612-1",
	GenesisURL:    "https://raw.githubusercontent.com/mezo-org/mezod/v11.0.1/chain/mainnet/mezo_31612-1/genesis.json",
	GenesisSHA256: "c1b9b2736bc0c1e6390dafa80c43fdc9870490d481ebaa71d932a98df3c531f7",
	Seeds:         "a44ea22836e79c2e8e2e64547243ce6925746e91@35.208.223.127:26656,3248ba5a691a6422c6bda443a43cfdf48e43cc85@mezo-mainnet-seed.validator.validationcloud.io:30232",
	StateSyncRPC:  []string{"https://rpc.lavenderfive.com:443/mezo", "https://rpc.lavenderfive.com:443/mezo"},
	Toolbox:       true,
}

// ContainerCommand initializes /data on first start; see cosmosNode.Command.
func (a *mezoAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return mezoNode.Command()
}

// InitContainers adds the busybox toolbox sidecar the start script runs on.
func (a *mezoAdapter) InitContainers(_ chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return mezoNode.InitContainers()
}

// ContainerArgs are `mezod start` flags: RPC must listen beyond localhost
// for the probes and HealthCheck.
func (a *mezoAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--rpc.laddr", "tcp://0.0.0.0:26657"}
}

func (a *mezoAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "api", ContainerPort: 1317, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "evm-rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "evm-ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck uses evmHealthCheck since mezod exposes EVM JSON-RPC at 8545.
func (a *mezoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *mezoAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *mezoAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "mezo/mezod",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const mezoConfig = `# Mezo node config (Cosmos SDK + EVMOS EVM extension)
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = false

[telemetry]
enabled = true
prometheus-retention-time = 60

[json-rpc]
enable = true
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
`

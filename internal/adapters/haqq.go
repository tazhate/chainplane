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

// haqqNode bootstraps haqq_11235-1. The image runs cosmovisor under tini
// with DAEMON_HOME in the image, so args alone were exec'd as a command;
// the script runs haqqd directly against /data. The geth-style --metrics
// flags are gone: CometBFT serves Prometheus on :26660 once
// [instrumentation] prometheus is on, which the script sets.
var haqqNode = cosmosNode{
	Binary:        "haqqd",
	ChainID:       "haqq_11235-1",
	GenesisURL:    "https://raw.githubusercontent.com/haqq-network/mainnet/733a7222cc4b40c2b20428e5428f255950971b9d/genesis.json",
	GenesisSHA256: "e381ec1785b8d53db036a54d5a4374a83530a1083116cdec568a1123afd0f8b1",
	Seeds:         "8542cd7e6bf9d260fef543bc49e59be5a3fa9074@seed.publicnode.com:26656",
	// Polkachu is listed twice: publicnode reports other proposer priorities
	// for the same height and the light client stops on the mismatch, and
	// the remaining chain-registry RPCs are down (2026-10-01).
	StateSyncRPC: []string{"https://haqq-rpc.polkachu.com:443", "https://haqq-rpc.polkachu.com:443"},
}

// ContainerCommand initializes /data on first start; see cosmosNode.Command.
func (a *haqqAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return haqqNode.Command()
}

// ContainerArgs are `haqqd start` flags.
func (a *haqqAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--rpc.laddr", "tcp://0.0.0.0:26657"}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// haqqConfig holds app.toml overrides, merged into the app.toml haqqd init
// generates. P2P and state sync settings live in config.toml, which the
// start script writes on first start.
const haqqConfig = `# Haqq Network (Islamic Coin) — Cosmos EVM chain node app.toml overrides
[json-rpc]
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
api = "eth,net,web3,txpool"
enable = true
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

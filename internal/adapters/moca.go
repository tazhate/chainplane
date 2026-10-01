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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// Moca Chain is Cosmos/EVMOS-based. The official image is ghcr.io/mocachain/mocad
// (entrypoint mocad). The default tracks the latest release, which suits a node
// restored from a snapshot; syncing from genesis may need to start at v1.2.1 and
// apply the later upgrades through cosmovisor.

// mocaBlockTime is the average Moca block interval used to estimate network tip.
const mocaBlockTime = 2.0 // seconds

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type mocaAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMoca, &mocaAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *mocaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMoca, client)
}

func (a *mocaAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", mocaConfig, nil
}

// mocaNode bootstraps moca_2288-1. Moca publishes no static genesis file
// (asset/configs/mainnet_config in mocachain/moca is the moca_5151-1
// devnet), so genesis is the /genesis response of the official RPC, pinned
// by its SHA-256. Seeds are empty upstream; the official sentry is the
// persistent peer, and both RPC hostnames reach the same node.
var mocaNode = cosmosNode{
	Binary:          "mocad",
	ChainID:         "moca_2288-1",
	GenesisURL:      "https://tm-rpc.mocachain.org/genesis",
	GenesisSHA256:   "22b93c2ec892c47f12516c1f7bbbcea457c0a1e601d5a40c279ec0825184e37f",
	GenesisFormat:   genesisRPC,
	PersistentPeers: "015fda7c14ddd2e74a29a3118e5c28c069219257@p2p.sentry-node-1.mocachain.dev:26656,d3e1ea4ad789bfc6c259019ae3ca13f05205f364@p2p.sentry-node-0.mocachain.dev:26656",
	StateSyncRPC:    []string{"https://tm-rpc.mocachain.org:443", "https://tm-rpc.mocachain.dev:443"},
}

// ContainerCommand initializes /data on first start; see cosmosNode.Command.
func (a *mocaAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return mocaNode.Command()
}

// ContainerArgs are `mocad start` flags: RPC must listen beyond localhost
// for the probes and HealthCheck, and mocad reads the chain ID from
// --chain-id or client.toml.
func (a *mocaAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--rpc.laddr", "tcp://0.0.0.0:26657", "--chain-id", "moca_2288-1"}
}

func (a *mocaAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "api", ContainerPort: 1317, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "evm-rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *mocaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("moca health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("moca health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("moca status parse: %w", err)
	}

	height := parseDecimalInt64(status.Result.SyncInfo.LatestBlockHeight)
	syncing := status.Result.SyncInfo.CatchingUp

	if height == 0 && syncing {
		return syncingPseudo(), nil
	}

	highestBlock := height
	if syncing {
		if blockTime, err := time.Parse(time.RFC3339Nano, status.Result.SyncInfo.LatestBlockTime); err == nil {
			lag := time.Since(blockTime).Seconds()
			if lag > mocaBlockTime {
				highestBlock = height + int64(lag/mocaBlockTime)
			}
		}
		if tip := cosmosNetworkTip(ctx, rpcURL); tip > highestBlock {
			highestBlock = tip
		}
	}

	var progress float64
	if !syncing {
		progress = 100.0
	} else {
		progress = progressFromBlocks(height, highestBlock)
	}

	var peers int32
	if req2, err2 := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/net_info", nil); err2 == nil {
		if resp2, err2 := (&http.Client{Timeout: 5 * time.Second}).Do(req2); err2 == nil {
			defer resp2.Body.Close()
			body2, _ := io.ReadAll(io.LimitReader(resp2.Body, maxResponseBytes))
			var netInfo struct {
				Result struct {
					NPeers string `json:"n_peers"`
				} `json:"result"`
			}
			if json.Unmarshal(body2, &netInfo) == nil {
				peers = int32(parseDecimalInt64(netInfo.Result.NPeers))
			}
		}
	}

	return SyncStatus{
		IsSyncing:    syncing,
		CurrentBlock: height,
		HighestBlock: highestBlock,
		Progress:     progress,
		Peers:        peers,
	}, nil
}

func (a *mocaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const mocaConfig = `# Moca Network node config (Cosmos/EVMOS-based)
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
`

func (a *mocaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "mocachain/mocad",
		TagPattern: `^v(?P<version>\d+\.\d+\.\d+)$`,
	}
}

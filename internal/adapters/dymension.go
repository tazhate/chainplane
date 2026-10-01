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

const (

	// dymensionBlockTime is the average Dymension block interval used to estimate network tip.
	dymensionBlockTime = 6.0 // seconds
)

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const dymensionConfig = `# Dymension modular rollup hub node config override
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = true
address = "0.0.0.0:9090"

[telemetry]
enabled = true
prometheus-retention-time = 60
`

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type dymensionAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainDymension, &dymensionAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *dymensionAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainDymension, client)
}

func (a *dymensionAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", dymensionConfig, nil
}

// dymensionNode bootstraps dymension_1100-1. The image has no ENTRYPOINT
// (CMD is /bin/sh), so the command must name dymd. Genesis is the Git LFS
// object of dymensionxyz/networks, pinned to a commit.
var dymensionNode = cosmosNode{
	Binary:          "dymd",
	ChainID:         "dymension_1100-1",
	GenesisURL:      "https://media.githubusercontent.com/media/dymensionxyz/networks/b721f0cc13da95f8a832e51f3ee886ded4e8676b/mainnet/dymension/genesis.json",
	GenesisSHA256:   "c25f362084db5c1480aaee93bfcb97c5328cabeda94f11ddcc74a8e183838491",
	Seeds:           "45bffa41836302b06310af67f012500cc0d1da31@rpc.dymension.nodestake.org:666,193262e32a9d7d3fffe14073160cabc4cdfef26b@dymension-rpc.stakeandrelax.net:20556,8542cd7e6bf9d260fef543bc49e59be5a3fa9074@seed.publicnode.com:26656",
	PersistentPeers: "e0d84deab2d0fd85f447c5c417fecbbdba584be0@dymension-m.peer.stavr.tech:17086,c600039ef70040740ae130d455768c509d173b12@peer.dymension.node75.org:23836,e3522d6de016578ac0935c4c55e13e4aac6f0693@peer.dymension.mainnet.dteam.tech:29656",
	StateSyncRPC:    []string{"https://dymension-rpc.polkachu.com:443", "https://rpc.lavenderfive.com:443/dymension"},
}

// ContainerCommand initializes /data on first start; see cosmosNode.Command.
func (a *dymensionAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return dymensionNode.Command()
}

// ContainerArgs are `dymd start` flags: RPC must listen beyond localhost
// for the probes and HealthCheck.
func (a *dymensionAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--rpc.laddr", "tcp://0.0.0.0:26657"}
}

func (a *dymensionAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 26656, Protocol: corev1.ProtocolUDP},
		{Name: "grpc", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *dymensionAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("dymension health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("dymension health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("dymension status parse: %w", err)
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
			if lag > dymensionBlockTime {
				highestBlock = height + int64(lag/dymensionBlockTime)
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

func (a *dymensionAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("300Gi"),
	}
}

func (a *dymensionAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "dymensionxyz/dymension",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

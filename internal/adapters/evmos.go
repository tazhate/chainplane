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

	// evmosBlockTime is the average Evmos block interval used to estimate network tip.
	evmosBlockTime = 2.0 // seconds
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type evmosAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainEvmos, &evmosAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *evmosAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainEvmos, client)
}

func (a *evmosAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", evmosConfig, nil
}

// evmosNode bootstraps evmos_9001-2. The image has no ENTRYPOINT, so the
// command must name evmosd. The chain halted at height 37,318,000 on
// 2026-05-18 after the shutdown proposal #331: archive.evmos.org is gone,
// so genesis comes from the Polkachu mirror, and with no live RPC left
// there is no state sync.
var evmosNode = cosmosNode{
	Binary:          "evmosd",
	ChainID:         "evmos_9001-2",
	GenesisURL:      "https://snapshots.polkachu.com/genesis/evmos/genesis.json",
	GenesisSHA256:   "4aa13da5eb4b9705ae8a7c3e09d1c36b92d08247dad2a6ed1844d031fcfe296c",
	Seeds:           "5740e4a36e646e80cc5648daf5e983e5b5d8f265@54.39.18.27:26656,40f4fac63da8b1ce8f850b0fa0f79b2699d2ce72@seed.evmos.jerrychong.com:26656,8542cd7e6bf9d260fef543bc49e59be5a3fa9074@seed.publicnode.com:26656",
	PersistentPeers: "5740e4a36e646e80cc5648daf5e983e5b5d8f265@54.39.18.27:26656",
}

// ContainerCommand initializes /data on first start; see cosmosNode.Command.
func (a *evmosAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return evmosNode.Command()
}

// ContainerArgs are `evmosd start` flags: RPC must listen beyond localhost
// for the probes and HealthCheck.
func (a *evmosAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--rpc.laddr", "tcp://0.0.0.0:26657"}
}

func (a *evmosAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "api", ContainerPort: 1317, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "evm-rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// StartupProbe gives Evmos up to 1h (120x30s) to complete state sync.
func (a *evmosAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(26657, 30, 30, 10, 120)
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *evmosAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("evmos health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("evmos health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("evmos status parse: %w", err)
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
			if lag > evmosBlockTime {
				highestBlock = height + int64(lag/evmosBlockTime)
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

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const evmosConfig = `# Evmos node config override
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

func (a *evmosAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

// Evmos halted for good on 2026-05-18 at height 37,318,000 after the
// shutdown proposal #331, so there is no default image and nothing to
// track. The adapter stays registered for existing ChainInstances that set
// spec.image.

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

	// osmosisBlockTime is the average Osmosis block interval used to estimate network tip.
	osmosisBlockTime = 6.0 // seconds
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type osmosisAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainOsmosis, &osmosisAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *osmosisAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainOsmosis, client)
}

func (a *osmosisAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", osmosisConfig, nil
}

// osmosisNode bootstraps osmosis-1. The image is distroless (ENTRYPOINT
// osmosisd, no shell), so the script runs on the busybox toolbox. Genesis
// is the Git LFS object of osmosis-labs/networks, pinned to a commit.
var osmosisNode = cosmosNode{
	Binary:          "osmosisd",
	ChainID:         "osmosis-1",
	GenesisURL:      "https://media.githubusercontent.com/media/osmosis-labs/networks/0e53c816e0fab7bc78a5f14f618fc5c8185e101e/osmosis-1/genesis.json",
	GenesisSHA256:   "1cdb76087fabcca7709fc563b44b5de98aaf297eedc8805aa2884999e6bab06d",
	Seeds:           "ade4d8bc8cbe014af6ebdf3cb7b1e9ad36f412c0@seeds.polkachu.com:12556,e891d42c31064fb7e0d99839536164473c4905c2@seed-osmosis.freshstaking.com:31656,8542cd7e6bf9d260fef543bc49e59be5a3fa9074@seed.publicnode.com:26656,b85358e035343a3b15e77e1102857dcdaf70053b@seeds.bluestake.net:24856",
	PersistentPeers: "e891d42c31064fb7e0d99839536164473c4905c2@seed-osmosis.freshstaking.com:31656",
	StateSyncRPC:    []string{"https://osmosis-rpc.polkachu.com:443", "https://osmosis-rpc.publicnode.com:443"},
	Toolbox:         true,
}

// ContainerCommand initializes /data on first start; see cosmosNode.Command.
func (a *osmosisAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return osmosisNode.Command()
}

// InitContainers adds the busybox toolbox sidecar the start script runs on.
func (a *osmosisAdapter) InitContainers(_ chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return osmosisNode.InitContainers()
}

// ContainerArgs are `osmosisd start` flags: RPC must listen beyond localhost
// for the probes and HealthCheck.
func (a *osmosisAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--rpc.laddr", "tcp://0.0.0.0:26657"}
}

func (a *osmosisAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "api", ContainerPort: 1317, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// StartupProbe gives Osmosis up to 1h (120x30s) to complete state sync.
func (a *osmosisAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(26657, 30, 30, 10, 120)
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *osmosisAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("osmosis health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("osmosis health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("osmosis status parse: %w", err)
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
			if lag > osmosisBlockTime {
				highestBlock = height + int64(lag/osmosisBlockTime)
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

const osmosisConfig = `# Osmosis node config override
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = false

[telemetry]
enabled = true
prometheus-retention-time = 60
`

func (a *osmosisAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

// VersionPolicy tracks osmolabs/osmosis, which tags releases without a "v"
// prefix. Docker Hub also carries 31.1.0, but it has no matching git tag in
// osmosis-labs/osmosis, so the default stays on 31.0.3 until its provenance is
// clear; review such bumps by hand.
func (a *osmosisAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "osmolabs/osmosis",
		TagPattern: `^(?P<version>\d+\.\d+\.\d+)$`,
	}
}

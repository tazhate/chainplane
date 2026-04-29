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

type starknetAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainStarknet, &starknetAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 6060},
	})
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const starknetConfig = `{
  "network": "mainnet",
  "http": true,
  "http-port": 6060,
  "http-host": "0.0.0.0",
  "db-path": "/data/juno",
  "metrics": true,
  "metrics-port": 9090,
  "p2p": true,
  "p2p-addr": "0.0.0.0:7777",
  "colour": false
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *starknetAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainStarknet, client)
}

func (a *starknetAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "juno.json", starknetConfig, nil
}

func (a *starknetAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// starknet_syncing returns false when synced, or a sync status object when syncing
	syncResult, err := callRPC(ctx, rpcURL, "starknet_syncing", nil)
	if err != nil {
		if isContextTimeout(err) {
			return syncingPseudo(), nil
		}
		return SyncStatus{}, fmt.Errorf("starknet_syncing: %w", err)
	}

	// If result is "false" (boolean), node is fully synced — get block number
	var syncing bool
	if err := json.Unmarshal(syncResult, &syncing); err == nil && !syncing {
		// Get current block number
		blockResult, err := callRPC(ctx, rpcURL, "starknet_blockNumber", nil)
		if err != nil {
			return SyncStatus{IsSyncing: false}, nil
		}
		var blockNum int64
		if err := json.Unmarshal(blockResult, &blockNum); err != nil {
			return SyncStatus{IsSyncing: false}, nil
		}
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: blockNum,
			HighestBlock: blockNum,
			Progress:     100.0,
		}, nil
	}

	// Node is syncing — parse the sync status object
	var syncStatus struct {
		CurrentBlockNum  int64  `json:"current_block_num"`
		HighestBlockNum  int64  `json:"highest_block_num"`
		CurrentBlockHash string `json:"current_block_hash"`
		HighestBlockHash string `json:"highest_block_hash"`
		StartingBlockNum int64  `json:"starting_block_num"`
	}
	if err := json.Unmarshal(syncResult, &syncStatus); err != nil {
		return syncingPseudo(), nil
	}

	progress := progressFromBlocks(syncStatus.CurrentBlockNum, syncStatus.HighestBlockNum)
	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: syncStatus.CurrentBlockNum,
		HighestBlock: syncStatus.HighestBlockNum,
		Progress:     progress,
	}, nil
}

// StartupProbe gives Starknet/Juno up to 3h (360x30s) to complete initial sync.
func (a *starknetAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(6060, 30, 30, 10, 360)
}

func (a *starknetAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	network := "mainnet"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		network = "sepolia"
	}
	return []string{
		"--network", network,
		"--http",
		"--http-port", "6060",
		"--http-host", "0.0.0.0",
		"--db-path", "/data/juno",
		"--metrics",
		"--metrics-port", "9090",
		"--p2p",
		"--p2p-addr", "0.0.0.0:7777",
		"--colour=false",
	}
}

func (a *starknetAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 7777, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
	}
}

func (a *starknetAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *starknetAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "nethermindeth/juno",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

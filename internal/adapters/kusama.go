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

type kusamaAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainKusama, &kusamaAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 9944},
	})
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const kusamaConfig = `{
  "name": "kusama-node",
  "rpc": {
    "port": 9944,
    "external": true,
    "cors": "all",
    "methods": "unsafe"
  },
  "ws": {
    "port": 9945,
    "external": true
  },
  "prometheus": {
    "port": 9615,
    "external": true
  },
  "network": {
    "port": 30333,
    "nodeKey": ""
  },
  "base": {
    "path": "/data",
    "chain": "kusama"
  }
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *kusamaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainKusama, client)
}

func (a *kusamaAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "kusama.json", kusamaConfig, nil
}

// HealthCheck uses the Substrate JSON-RPC protocol (same as Polkadot).
// Calls system_syncState to get block progress and system_health to get peers.
func (a *kusamaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return substrateHealthCheck(ctx, rpcURL)
}

// StartupProbe gives Kusama up to 2h (240x30s) to complete warp sync.
func (a *kusamaAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(9944, 30, 30, 10, 240)
}

func (a *kusamaAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	chain := "kusama"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		chain = "westend"
	}
	return []string{
		"--base-path", "/data",
		"--chain", chain,
		"--rpc-port", "9944",
		"--rpc-external",
		"--rpc-cors", "all",
		"--rpc-methods", "unsafe",
		"--ws-port", "9945",
		"--ws-external",
		"--prometheus-port", "9615",
		"--prometheus-external",
		"--port", "30333",
		"--name", "k8s-kusama-node",
		"--no-hardware-benchmarks",
		"--state-pruning", "256",
	}
}

func (a *kusamaAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9944, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9945, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30333, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30333, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 9615, Protocol: corev1.ProtocolTCP},
	}
}

func (a *kusamaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *kusamaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "parity/polkadot",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Substrate health check (shared helper)
// --------------------------------------------------------------------------

// substrateHealthCheck performs a health check for Substrate-based chains
// (Polkadot, Kusama, etc.) using system_syncState and system_health RPC methods.
func substrateHealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// system_health returns {"isSyncing": bool, "peers": int, "shouldHavePeers": bool}
	healthResult, err := callRPC(ctx, rpcURL, "system_health", nil)
	if err != nil {
		if isContextTimeout(err) {
			return syncingPseudo(), nil
		}
		return SyncStatus{}, fmt.Errorf("system_health: %w", err)
	}

	var health struct {
		IsSyncing bool  `json:"isSyncing"`
		Peers     int32 `json:"peers"`
	}
	if err := json.Unmarshal(healthResult, &health); err != nil {
		return SyncStatus{}, fmt.Errorf("parse system_health: %w", err)
	}

	// system_syncState returns {"currentBlock":N,"highestBlock":N,"startingBlock":N}
	syncResult, err := callRPC(ctx, rpcURL, "system_syncState", nil)
	if err != nil {
		// Best-effort: return health status without block height detail
		if health.IsSyncing {
			return syncingPseudo(), nil
		}
		return SyncStatus{IsSyncing: false, Peers: health.Peers}, nil
	}

	var syncState struct {
		CurrentBlock  int64 `json:"currentBlock"`
		HighestBlock  int64 `json:"highestBlock"`
		StartingBlock int64 `json:"startingBlock"`
	}
	if err := json.Unmarshal(syncResult, &syncState); err != nil {
		return SyncStatus{}, fmt.Errorf("parse system_syncState: %w", err)
	}

	current := syncState.CurrentBlock
	highest := syncState.HighestBlock
	if highest <= 0 {
		highest = current
	}

	if !health.IsSyncing {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: current,
			HighestBlock: highest,
			Progress:     100.0,
			Peers:        health.Peers,
		}, nil
	}

	progress := progressFromBlocks(current, highest)
	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: current,
		HighestBlock: highest,
		Progress:     progress,
		Peers:        health.Peers,
	}, nil
}

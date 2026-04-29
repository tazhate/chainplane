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

type filecoinAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainFilecoin, &filecoinAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 1234},
	})
}

// --------------------------------------------------------------------------
// Config (static TOML for Lotus)
// --------------------------------------------------------------------------

const filecoinConfig = `[API]
  ListenAddress = "/ip4/0.0.0.0/tcp/1234/http"
  RemoteListenAddress = ""
  Timeout = "30s"

[Libp2p]
  ListenAddresses = ["/ip4/0.0.0.0/tcp/12345"]
  ConnMgrLow = 150
  ConnMgrHigh = 180
  ConnMgrGrace = "20s"

[Chainstore]
  EnableSplitstore = true

[Chainstore.Splitstore]
  HotStoreType = "badger"
  MarkSetType = "badger"
  HotStoreFullGCFrequency = 20
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *filecoinAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainFilecoin, client)
}

func (a *filecoinAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", filecoinConfig, nil
}

func (a *filecoinAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// Filecoin.ChainHead returns the current tipset with Height field
	headResult, err := callRPC(ctx, rpcURL, "Filecoin.ChainHead", nil)
	if err != nil {
		if isContextTimeout(err) {
			return syncingPseudo(), nil
		}
		return SyncStatus{}, fmt.Errorf("Filecoin.ChainHead: %w", err)
	}

	var tipset struct {
		Height int64 `json:"Height"`
	}
	if err := json.Unmarshal(headResult, &tipset); err != nil {
		return SyncStatus{}, fmt.Errorf("parse Filecoin.ChainHead: %w", err)
	}

	// Filecoin.SyncState returns sync workers with their status
	syncResult, err := callRPC(ctx, rpcURL, "Filecoin.SyncState", nil)
	if err != nil {
		// Best-effort: return chain head without sync state
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: tipset.Height,
			HighestBlock: tipset.Height,
			Progress:     100.0,
		}, nil
	}

	var syncState struct {
		ActiveSyncs []struct {
			Stage  int   `json:"Stage"`
			Height int64 `json:"Height"`
			Target *struct {
				Height int64 `json:"Height"`
			} `json:"Target"`
		} `json:"ActiveSyncs"`
	}
	if err := json.Unmarshal(syncResult, &syncState); err != nil {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: tipset.Height,
			HighestBlock: tipset.Height,
			Progress:     100.0,
		}, nil
	}

	// Check if any sync worker is actively syncing (Stage < 4 means not complete)
	var isSyncing bool
	var highestTarget int64
	for _, sync := range syncState.ActiveSyncs {
		if sync.Stage < 4 && sync.Target != nil {
			isSyncing = true
			if sync.Target.Height > highestTarget {
				highestTarget = sync.Target.Height
			}
		}
	}

	if !isSyncing || highestTarget == 0 {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: tipset.Height,
			HighestBlock: tipset.Height,
			Progress:     100.0,
		}, nil
	}

	progress := progressFromBlocks(tipset.Height, highestTarget)
	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: tipset.Height,
		HighestBlock: highestTarget,
		Progress:     progress,
	}, nil
}

// StartupProbe gives Filecoin/Lotus up to 6h (720x30s) to complete initial sync.
func (a *filecoinAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(1234, 60, 30, 10, 720)
}

func (a *filecoinAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	args := []string{
		"daemon",
		"--repo=/data",
	}
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		args = append(args, "--chain=calibnet")
	}
	return args
}

func (a *filecoinAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("32Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

func (a *filecoinAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "filecoin/lotus",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *filecoinAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 1234, Protocol: corev1.ProtocolTCP},
		// Lotus exposes Prometheus metrics at /debug/metrics on the API port
		{Name: "metrics", ContainerPort: 1234, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 12345, Protocol: corev1.ProtocolTCP},
	}
}

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const (
	rethMetricsPort       = int32(9001)
	gethMetricsPort       = int32(6060)
	erigonMetricsPort     = int32(6060)
	nethermindMetricsPort = int32(9091)
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type ethereumAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	eth := &ethereumAdapter{protocolAdapter: protocolAdapter{livenessPort: 8545}}
	Register(chainsv1alpha2.ChainEthereum, eth)
	Register(chainsv1alpha2.ChainEthereumArchive, eth)
}

// --------------------------------------------------------------------------
// Config templates (parsed once at init time)
// --------------------------------------------------------------------------

var ethConfigTemplates = map[string]*template.Template{
	"geth": template.Must(template.New("geth").Parse(`# Geth config for {{ .Chain }} {{ .Network }}
[Eth]
NetworkId = 1

[Node]
DataDir = "/data"

[Node.P2P]
MaxPeers = 50

[Node.HTTPTimeouts]
ReadTimeout = "30s"
`)),
	"reth": template.Must(template.New("reth").Parse(`# Reth config for {{ .Chain }} {{ .Network }}
[network]
chain = "{{ .Network }}"

[rpc]
http = true
http-addr = "0.0.0.0"
http-port = 8545
ws = true
ws-addr = "0.0.0.0"
ws-port = 8546
`)),
	"erigon": template.Must(template.New("erigon").Parse(`# Erigon config for {{ .Chain }} {{ .Network }}
[main]
chain = "{{ .Network }}"
datadir = "/data"
http = true
http.addr = "0.0.0.0"
http.port = 8545
http.vhosts = "*"
ws = true
`)),
	"nethermind": template.Must(template.New("nethermind").Parse(`{
  "Init": {
    "Network": "{{ .Network }}",
    "WebSocketsEnabled": true,
    "BaseDbPath": "/data"
  },
  "JsonRpc": {
    "Enabled": true,
    "Host": "0.0.0.0",
    "Port": 8545,
    "WebSocketsPort": 8546
  },
  "Sync": {
    "FastSync": {{ .FastSync }}
  },
  "Metrics": {
    "Enabled": true,
    "ExposePort": 9091
  }
}`)),
}

var ethConfigFilenames = map[string]string{
	"geth":       "config.toml",
	"reth":       "reth.toml",
	"erigon":     "erigon.yaml",
	"nethermind": "nethermind.json",
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *ethereumAdapter) DefaultImage(client string) string {
	// v1.15+ changed DB format (PBSS). Existing geth nodes must resync when upgrading from v1.14.x.
	return DefaultImageFor(chainsv1alpha2.ChainEthereum, client)
}

func (a *ethereumAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	network := "mainnet"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		network = "sepolia"
	}

	client := strings.ToLower(spec.Client)
	tpl, ok := ethConfigTemplates[client]
	if !ok {
		tpl = ethConfigTemplates["nethermind"]
		client = "nethermind"
	}

	var buf bytes.Buffer
	err := tpl.Execute(&buf, struct {
		Chain    chainsv1alpha2.Chain
		Network  string
		FastSync bool
	}{
		Chain:    spec.Chain,
		Network:  network,
		FastSync: spec.NodeType != chainsv1alpha2.NodeTypeArchive,
	})
	if err != nil {
		return "", "", fmt.Errorf("ethereum config template (%s): %w", client, err)
	}

	return ethConfigFilenames[client], buf.String(), nil
}

func (a *ethereumAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// Call eth_syncing — may timeout during heavy stage work (IndexStorageHistory, etc.)
	syncResult, syncErr := callRPC(ctx, rpcURL, "eth_syncing", nil)

	// Always try eth_blockNumber even if eth_syncing timed out — it's a lighter call.
	var currentBlock int64
	if blockResult, blockErr := callRPC(ctx, rpcURL, "eth_blockNumber", nil); blockErr == nil {
		var blockHex string
		if json.Unmarshal(blockResult, &blockHex) == nil {
			currentBlock = hexToInt64(blockHex)
		}
	}

	if syncErr != nil {
		if isContextTimeout(syncErr) {
			// RPC timeout during heavy stage work (SenderRecovery, Execution, IndexStorageHistory, etc.)
			// is normal for Reth/Erigon and must not trigger a Degraded restart.
			return SyncStatus{
				IsSyncing:    true,
				CurrentBlock: currentBlock,
				HighestBlock: currentBlock,
				Progress:     100.0,
				StallExempt:  true,
			}, nil
		}
		return SyncStatus{}, fmt.Errorf("eth_syncing: %w", syncErr)
	}

	peers := evmPeerCount(ctx, rpcURL)

	var syncingFalse bool
	if err := json.Unmarshal(syncResult, &syncingFalse); err == nil && !syncingFalse {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: currentBlock,
			HighestBlock: currentBlock,
			Progress:     100.0,
			Peers:        peers,
		}, nil
	}

	var syncObj struct {
		CurrentBlock string `json:"currentBlock"`
		HighestBlock string `json:"highestBlock"`
		Stages       []struct {
			StageName   string `json:"name"`
			BlockNumber string `json:"block"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(syncResult, &syncObj); err != nil {
		return SyncStatus{}, fmt.Errorf("parse eth_syncing: %w", err)
	}

	// Polygon-specific stages that are always 0 and must be excluded.
	polygonStages := map[string]bool{"BorHeimdall": true, "Translation": true}

	// Erigon post-sync indexing stages: blocks are fully downloaded but indexing lags.
	// Treat as synced (current = highest) to avoid false stall detection.
	postSyncStages := map[string]bool{
		"IndexStorageHistory":   true,
		"IndexAccountHistory":   true,
		"IndexLogAddressTopics": true,
		"LogIndex":              true,
		"TxLookup":              true,
		"CallTraces":            true,
		"Finish":                true,
	}

	var current, highest int64
	if len(syncObj.Stages) > 0 {
		current = -1 // sentinel
		var bottleneckStage string
		var bottleneckBlock int64
		for _, s := range syncObj.Stages {
			if polygonStages[s.StageName] {
				continue
			}
			n := hexToInt64(s.BlockNumber)
			if n > highest {
				highest = n
			}
			if current < 0 || n < current {
				current = n
				bottleneckStage = s.StageName
				bottleneckBlock = n
			}
		}
		if current < 0 {
			current = 0
		}
		_ = bottleneckBlock

		if highest > 0 && postSyncStages[bottleneckStage] {
			// Blocks are all synced; only post-sync indexing remains.
			current = highest
		}
	} else {
		current = currentBlock
		highest = hexToInt64(syncObj.HighestBlock)
	}

	progress := progressFromBlocks(current, highest)

	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: current,
		HighestBlock: highest,
		Progress:     progress,
		Peers:        peers,
		StallExempt:  true,
	}, nil
}

func (a *ethereumAdapter) LivenessProbe(spec chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	port := int32(8545)
	if spec.RPC.Port > 0 {
		port = spec.RPC.Port
	}
	return tcpProbe(port, 300, 30, 10, 3) // Reth/Erigon startup time before RPC ready
}

func (a *ethereumAdapter) ContainerPorts(spec chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name:          "metrics",
		ContainerPort: ethMetricsPort(spec.Client),
		Protocol:      corev1.ProtocolTCP,
	})
}

// ContainerArgs injects the per-client Prometheus metrics flag so that the
// /metrics endpoint is always available for health checks and PodMonitor scraping.
func (a *ethereumAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return ethMetricsArgs(spec.Client)
}

// ethMetricsPort returns the Prometheus metrics port for the given EVM client.
func ethMetricsPort(client string) int32 {
	switch strings.ToLower(client) {
	case "reth":
		return rethMetricsPort
	case "erigon":
		return erigonMetricsPort
	case "nethermind":
		return nethermindMetricsPort
	default: // geth and unspecified
		return gethMetricsPort
	}
}

// ethMetricsArgs returns the CLI flags needed to expose Prometheus metrics for
// the given EVM client. Nethermind uses JSON config (see ConfigTemplate) and
// needs no extra args.
func ethMetricsArgs(client string) []string {
	switch strings.ToLower(client) {
	case "reth":
		return []string{"node", "--metrics", "0.0.0.0:9001"}
	case "erigon":
		return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
	case "nethermind":
		return nil // metrics configured via JSON config template
	default: // geth
		return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
	}
}

func (a *ethereumAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "nethermind/nethermind",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

func (a *ethereumAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

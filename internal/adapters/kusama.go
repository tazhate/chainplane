package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultKusamaImage = "parity/polkadot:v1.15.1"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type kusamaAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainKusama, &kusamaAdapter{
		baseAdapter: baseAdapter{livenessPort: 9944},
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

func (a *kusamaAdapter) DefaultImage(_ string) string {
	return defaultKusamaImage
}

func (a *kusamaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "kusama.json", kusamaConfig, nil
}

// HealthCheck uses the Substrate JSON-RPC protocol (same as Polkadot).
// Calls system_syncState to get block progress and system_health to get peers.
func (a *kusamaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return substrateHealthCheck(ctx, rpcURL)
}

// StartupProbe gives Kusama up to 2h (240x30s) to complete warp sync.
func (a *kusamaAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(9944, 30, 30, 10, 240)
}

func (a *kusamaAdapter) ContainerArgs(spec nodesv1alpha1.BlockchainNodeSpec) []string {
	chain := "kusama"
	if spec.Network == nodesv1alpha1.NetworkTestnet {
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

func (a *kusamaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9944, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9945, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30333, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30333, Protocol: corev1.ProtocolUDP},
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
		CurrentBlock int64 `json:"currentBlock"`
		HighestBlock int64 `json:"highestBlock"`
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

package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultPolkadotImage = "parity/polkadot:v1.16.2"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type polkadotAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainPolkadot, &polkadotAdapter{
		baseAdapter: baseAdapter{livenessPort: 9944},
	})
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const polkadotConfig = `{
  "name": "polkadot-node",
  "rpc": {
    "port": 9944,
    "external": true,
    "cors": "all",
    "methods": "unsafe"
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
    "chain": "polkadot"
  }
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *polkadotAdapter) DefaultImage(_ string) string {
	return defaultPolkadotImage
}

func (a *polkadotAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "polkadot.json", polkadotConfig, nil
}

func (a *polkadotAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// system_health returns {"isSyncing": bool, "peers": int, "shouldHavePeers": bool}
	result, err := callRPC(ctx, rpcURL, "system_health", nil)
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
	if err := json.Unmarshal(result, &health); err != nil {
		return SyncStatus{}, fmt.Errorf("parse system_health: %w", err)
	}

	// Get block height via chain_getHeader (latest finalized)
	headerResult, err := callRPC(ctx, rpcURL, "chain_getHeader", nil)
	if err != nil {
		// Best-effort: return health status without block height
		if health.IsSyncing {
			return syncingPseudo(), nil
		}
		return SyncStatus{IsSyncing: false, Peers: health.Peers}, nil
	}

	var header struct {
		Number string `json:"number"` // hex-encoded, e.g. "0x1a2b3c"
	}
	if err := json.Unmarshal(headerResult, &header); err != nil {
		return SyncStatus{}, fmt.Errorf("parse chain_getHeader: %w", err)
	}

	blockHeight := polkadotParseHexBlock(header.Number)

	if !health.IsSyncing {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: blockHeight,
			HighestBlock: blockHeight,
			Progress:     100.0,
			Peers:        health.Peers,
		}, nil
	}

	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: blockHeight,
		HighestBlock: blockHeight + 1,
		Progress:     0,
		Peers:        health.Peers,
	}, nil
}

// StartupProbe gives Polkadot up to 2h (240x30s) to complete warp sync.
func (a *polkadotAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(9944, 30, 30, 10, 240)
}

func (a *polkadotAdapter) ContainerArgs(spec nodesv1alpha1.BlockchainNodeSpec) []string {
	chain := "polkadot"
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
		"--prometheus-port", "9615",
		"--prometheus-external",
		"--port", "30333",
		"--name", "k8s-polkadot-node",
		"--no-hardware-benchmarks",
		"--state-pruning", "256",
	}
}

func (a *polkadotAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9944, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 30333, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9615, Protocol: corev1.ProtocolTCP},
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func (a *polkadotAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *polkadotAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "parity/polkadot",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// polkadotParseHexBlock parses a hex string like "0x1a2b3c" to int64.
func polkadotParseHexBlock(hex string) int64 {
	hex = strings.TrimPrefix(hex, "0x")
	hex = strings.TrimPrefix(hex, "0X")
	n, _ := strconv.ParseInt(hex, 16, 64)
	return n
}

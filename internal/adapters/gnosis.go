package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultGnosisImage = "nethermind/nethermind:1.36.1"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type gnosisAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainGnosis, &gnosisAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *gnosisAdapter) DefaultImage(_ string) string {
	return defaultGnosisImage
}

func (a *gnosisAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "nethermind.json", gnosisConfig, nil
}

func (a *gnosisAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *gnosisAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *gnosisAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

func (a *gnosisAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"--config", "gnosis",
		"--datadir", "/data",
		"--JsonRpc.Enabled", "true",
		"--JsonRpc.Host", "0.0.0.0",
		"--JsonRpc.Port", "8545",
		"--JsonRpc.WebSocketsPort", "8546",
		"--JsonRpc.EnabledModules", "[Eth,Net,Web3,Subscribe,Health]",
		"--Network.P2PPort", "30303",
		"--Network.DiscoveryPort", "30303",
		"--HealthChecks.Enabled", "true",
	}
}

// --------------------------------------------------------------------------
// Config (Nethermind JSON for Gnosis network)
// --------------------------------------------------------------------------

const gnosisConfig = `{
  "Init": {
    "ChainSpecPath": "chainspec/gnosis.json",
    "BaseDbPath": "/data/db",
    "LogDirectory": "/data/logs"
  },
  "JsonRpc": {
    "Enabled": true,
    "Host": "0.0.0.0",
    "Port": 8545,
    "WebSocketsPort": 8546,
    "EnabledModules": ["Eth", "Net", "Web3", "Subscribe", "Health"]
  },
  "Network": {
    "P2PPort": 30303,
    "DiscoveryPort": 30303
  },
  "HealthChecks": {
    "Enabled": true
  },
  "Sync": {
    "FastSync": true
  }
}
`

package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
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
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
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
		"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060",
	}
}

func (a *gnosisAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *gnosisAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "nethermind/nethermind",
		TagPattern: `^\d+\.\d+\.\d+$`,
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

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

const defaultLineaL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type lineaAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainLinea, &lineaAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Config (Besu TOML)
// --------------------------------------------------------------------------

const lineaConfig = `# Linea Besu node configuration
data-path="/data"
network="linea"
sync-mode="SNAP"

rpc-http-enabled=true
rpc-http-host="0.0.0.0"
rpc-http-port=8545
rpc-http-api=["ETH","NET","WEB3","TXPOOL","LINEA"]
rpc-http-cors-origins=["*"]

rpc-ws-enabled=true
rpc-ws-host="0.0.0.0"
rpc-ws-port=8546
rpc-ws-api=["ETH","NET","WEB3"]

host-allowlist=["*"]
p2p-host="0.0.0.0"
p2p-port=30303
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *lineaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainLinea, client)
}

func (a *lineaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", lineaConfig, nil
}

func (a *lineaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// ContainerEnv injects the L1 Ethereum RPC URL required by Linea Besu node.
func (a *lineaAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultLineaL1URL},
	}
}

// ContainerArgs provides CLI flags including the L1 RPC endpoint and Prometheus metrics.
func (a *lineaAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"--config-file=/config/config.toml",
		"--plugin-linea-l1-rpc-endpoint=$(L1_RPC_URL)",
		"--metrics-enabled",
		"--metrics-host=0.0.0.0",
		"--metrics-port=9545",
	}
}

func (a *lineaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

func (a *lineaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "consensys/linea-besu",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

func (a *lineaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 9545, Protocol: corev1.ProtocolTCP,
	})
}

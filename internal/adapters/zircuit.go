package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultZircuitImage = "ghcr.io/zircuit-labs/l2-geth-public:v1.0.0"

const defaultZircuitL1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type zircuitAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainZircuit, &zircuitAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *zircuitAdapter) DefaultImage(_ string) string {
	return defaultZircuitImage
}

func (a *zircuitAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", zircuitConfig, nil
}

func (a *zircuitAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *zircuitAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// ContainerEnv injects the L1_RPC_URL environment variable required by Zircuit (L2).
func (a *zircuitAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultZircuitL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const zircuitConfig = `# Zircuit node configuration
[Eth]
SyncMode = "snap"

[Node]
DataDir = "/data"

[Node.HTTPHost]
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPVirtualHosts = ["*"]
HTTPCorsDomain = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool"]

[Node.WSHost]
WSHost = "0.0.0.0"
WSPort = 8546
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

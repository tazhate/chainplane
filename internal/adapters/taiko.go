package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultTaikoImage = "taikoxyz/taiko-geth:v1.8.0"
const defaultTaikoL1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type taikoAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainTaiko, &taikoAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *taikoAdapter) DefaultImage(_ string) string {
	return defaultTaikoImage
}

func (a *taikoAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", taikoConfig, nil
}

func (a *taikoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *taikoAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// ContainerEnv injects the L1_RPC_URL environment variable required by Taiko L2 nodes.
func (a *taikoAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultTaikoL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const taikoConfig = `# Taiko L2 geth node
[Eth]
NetworkId = 167000
SyncMode = "snap"

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
`

package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultGoatImage = "ghcr.io/goatnetwork/goat-geth:v0.4.2"

const defaultGoatL1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type goatAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainGoat, &goatAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *goatAdapter) DefaultImage(_ string) string {
	return defaultGoatImage
}

func (a *goatAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", goatConfig, nil
}

func (a *goatAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *goatAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// ContainerEnv injects the L1_RPC_URL required by the Goat Bitcoin L2 node.
// NOTE: GOAT Network is a Bitcoin L2 with Ethereum settlement. L1_RPC_URL here points to Ethereum for state derivation.
func (a *goatAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultGoatL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const goatConfig = `# Goat Network (Bitcoin L2 with EVM) node configuration
[Node]
DataDir = "/data"

[Node.HTTP]
Host = "0.0.0.0"
Port = 8545
VirtualHosts = ["*"]
Modules = ["eth", "net", "web3", "debug"]

[Node.WS]
Host = "0.0.0.0"
Port = 8546
Origins = ["*"]
Modules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

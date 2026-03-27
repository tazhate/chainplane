package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultKromaImage = "kromanetwork/geth:v0.5.0"

const defaultKromaL1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type kromaAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainKroma, &kromaAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *kromaAdapter) DefaultImage(_ string) string {
	return defaultKromaImage
}

func (a *kromaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", kromaConfig, nil
}

func (a *kromaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *kromaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// ContainerEnv injects the L1_RPC_URL environment variable required by the Kroma OP Stack node.
func (a *kromaAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultKromaL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const kromaConfig = `# op-geth configuration for Kroma (OP Stack L2)
[Eth]
NetworkId = 255
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

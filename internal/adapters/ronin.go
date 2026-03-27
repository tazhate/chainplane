package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultRoninImage = "ghcr.io/ronin-chain/ronin:v2.8.3"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type roninAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainRonin, &roninAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *roninAdapter) DefaultImage(_ string) string {
	return defaultRoninImage
}

func (a *roninAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", roninConfig, nil
}

func (a *roninAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *roninAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const roninConfig = `# Ronin mainnet RPC node
[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
`

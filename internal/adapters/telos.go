package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultTelosImage = "telosnetwork/telos-evm-rpc:v2.0.0"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type telosAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainTelos, &telosAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *telosAdapter) DefaultImage(_ string) string {
	return defaultTelosImage
}

func (a *telosAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "telos.toml", telosConfig, nil
}

func (a *telosAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *telosAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const telosConfig = `# Telos EVM RPC node configuration
[rpc]
host = "0.0.0.0"
port = 8545
ws_port = 8546

[p2p]
port = 30303

[database]
path = "/data"
`

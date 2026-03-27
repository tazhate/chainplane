package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// NOTE: aurora-relayer is deprecated. This uses standalone-rpc (nearaurora/srpc2-relayer). Full Aurora setup requires refiner + nearcore sidecars.
const defaultAuroraImage = "nearaurora/srpc2-relayer:latest"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type auroraAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainAurora, &auroraAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *auroraAdapter) DefaultImage(_ string) string {
	return defaultAuroraImage
}

func (a *auroraAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "aurora.toml", auroraConfig, nil
}

func (a *auroraAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *auroraAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const auroraConfig = `# Aurora (EVM on NEAR Protocol) relayer configuration
[server]
host = "0.0.0.0"
port = 8545

[near]
network = "mainnet"

[database]
path = "/data"
`

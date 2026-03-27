package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultPolygonZkEVMImage = "0xpolygonhermez/zkevm-node:v0.7.0"

const defaultPolygonZkEVML1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type polygonZkEVMAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainPolygonZkEVM, &polygonZkEVMAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *polygonZkEVMAdapter) DefaultImage(_ string) string {
	return defaultPolygonZkEVMImage
}

func (a *polygonZkEVMAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", polygonZkEVMConfig, nil
}

func (a *polygonZkEVMAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const polygonZkEVMConfig = `# Polygon zkEVM (CDK Validium) node configuration
[Node]
DataDir = "/data"

[Node.StateDB]
Host = "localhost"
Port = 5432

[Node.Executor]
URI = "localhost:50071"

[RPC]
Host = "0.0.0.0"
Port = 8545
WebSockets = true
WebSocketsPort = 8546
ReadTimeout = "60s"
WriteTimeout = "60s"
MaxRequestsPerIPAndSecond = 500

[Synchronizer]
SyncInterval = "1s"
SyncChunkSize = 100
`

func (a *polygonZkEVMAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

// ContainerEnv injects the L1_RPC_URL environment variable required by the zkEVM node.
func (a *polygonZkEVMAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultPolygonZkEVML1URL},
	}
}

package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultZeroNetworkImage = "matterlabs/external-node:v24.5.0"

const defaultZeroNetworkL1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type zeroNetworkAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainZeroNetwork, &zeroNetworkAdapter{
		baseAdapter: baseAdapter{livenessPort: 3060},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *zeroNetworkAdapter) DefaultImage(_ string) string {
	return defaultZeroNetworkImage
}

func (a *zeroNetworkAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "", "", nil
}

func (a *zeroNetworkAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *zeroNetworkAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 3060, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 3061, Protocol: corev1.ProtocolTCP},
	}
}

// ContainerEnv injects the L1 Ethereum RPC URL required by ZK Stack external node (ZERO Network).
func (a *zeroNetworkAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "EN_L1_ETH_CLIENT_WEB3_URL", Value: defaultZeroNetworkL1URL},
	}
}

package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultCronosZkEVMImage = "ghcr.io/cronos-labs/external-node:mainnet-v29.6.0"

const defaultCronosZkEVML1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type cronosZkEVMAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainCronosZkEVM, &cronosZkEVMAdapter{
		baseAdapter: baseAdapter{livenessPort: 3060},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *cronosZkEVMAdapter) DefaultImage(_ string) string {
	return defaultCronosZkEVMImage
}

func (a *cronosZkEVMAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "", "", nil
}

func (a *cronosZkEVMAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *cronosZkEVMAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 3060, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 3061, Protocol: corev1.ProtocolTCP},
	}
}

// ContainerEnv injects the L1 Ethereum RPC URL required by Cronos zkEVM ZK Stack external node.
func (a *cronosZkEVMAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "EN_L1_ETH_CLIENT_WEB3_URL", Value: defaultCronosZkEVML1URL},
	}
}

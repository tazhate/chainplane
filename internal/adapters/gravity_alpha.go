package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultGravityAlphaImage = "ghcr.io/celestiaorg/nitro:v3.6.8"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type gravityAlphaAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainGravityAlpha, &gravityAlphaAdapter{
		baseAdapter: baseAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *gravityAlphaAdapter) DefaultImage(_ string) string {
	return defaultGravityAlphaImage
}

func (a *gravityAlphaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "", "", nil
}

func (a *gravityAlphaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *gravityAlphaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8547, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8548, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30301, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30301, Protocol: corev1.ProtocolUDP},
	}
}

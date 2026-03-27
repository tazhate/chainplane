package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultPlumeImage = "public.ecr.aws/i6b2w2n6/nitro-node:plume-v2.3.2"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type plumeAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainPlume, &plumeAdapter{
		baseAdapter: baseAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *plumeAdapter) DefaultImage(_ string) string {
	return defaultPlumeImage
}

func (a *plumeAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "", "", nil
}

func (a *plumeAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *plumeAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8547, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8548, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30301, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30301, Protocol: corev1.ProtocolUDP},
	}
}

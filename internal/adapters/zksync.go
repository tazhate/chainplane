package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultZkSyncImage = "matterlabs/external-node:v2.0.22"

const defaultZkSyncL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type zksyncAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainZkSync, &zksyncAdapter{
		baseAdapter: baseAdapter{livenessPort: 3060},
	})
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const zksyncConfig = `# zkSync Era external node configuration
# RPC is exposed on port 3060 by default
EN_HTTP_PORT=3060
EN_WS_PORT=3061
EN_HEALTHCHECK_PORT=3071
EN_STATE_CACHE_PATH=/data/state_keeper
EN_MERKLE_TREE_PATH=/data/tree
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *zksyncAdapter) DefaultImage(_ string) string {
	return defaultZkSyncImage
}

func (a *zksyncAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "external-node.env", zksyncConfig, nil
}

func (a *zksyncAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// ContainerEnv injects the L1 Ethereum RPC URL required by zkSync Era external node.
func (a *zksyncAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "EN_ETH_CLIENT_URL", Value: defaultZkSyncL1URL},
	}
}

func (a *zksyncAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

func (a *zksyncAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "matterlabs/external-node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *zksyncAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 3060, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 3061, Protocol: corev1.ProtocolTCP},
		{Name: "health", ContainerPort: 3071, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 3312, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30303, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30303, Protocol: corev1.ProtocolUDP},
	}
}

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

const defaultPolygonZkEVMImage = "0xpolygonhermez/zkevm-node:v0.7.0"

const defaultPolygonZkEVML1URL = "http://ethereum:8545"

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

[Metrics]
Host = "0.0.0.0"
Port = 9091
Enabled = true

[Synchronizer]
SyncInterval = "1s"
SyncChunkSize = 100
`

func (a *polygonZkEVMAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	ports := evmPorts(30303)
	return append(ports, corev1.ContainerPort{Name: "metrics", ContainerPort: 9091, Protocol: corev1.ProtocolTCP})
}

// ContainerEnv injects the L1_RPC_URL environment variable required by the zkEVM node.
func (a *polygonZkEVMAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultPolygonZkEVML1URL},
	}
}

func (a *polygonZkEVMAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *polygonZkEVMAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "0xpolygonhermez/zkevm-node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

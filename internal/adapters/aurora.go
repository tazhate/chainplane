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

// NOTE: aurora-relayer is deprecated. This uses standalone-rpc (nearaurora/srpc2-relayer). Full Aurora setup requires refiner + nearcore sidecars.

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

func (a *auroraAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainAurora, client)
}

func (a *auroraAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "aurora.toml", auroraConfig, nil
}

func (a *auroraAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *auroraAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	ports := evmPorts(30303)
	// srpc2-relayer exposes Prometheus metrics on port 9090
	return append(ports, corev1.ContainerPort{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP})
}

func (a *auroraAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
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

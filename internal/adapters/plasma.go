package adapters

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// NOTE: Plasma Next (plasma.to) is a ZKP-based payment channel protocol, not a
// traditional full-node chain. This adapter is a placeholder; correct
// implementation requires Plasma-specific client.

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// plasma-next/node:v0.1.0 is a placeholder image — no public node image exists yet.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type plasmaAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainPlasma, &plasmaAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *plasmaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainPlasma, client)
}

func (a *plasmaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", plasmaConfig, nil
}

func (a *plasmaAdapter) HealthCheck(_ context.Context, _ string) (SyncStatus, error) {
	return SyncStatus{}, fmt.Errorf("plasma adapter not implemented")
}

func (a *plasmaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *plasmaAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *plasmaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *plasmaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "plasma-next/node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const plasmaConfig = `# Plasma Next placeholder configuration
# NOTE: Plasma Next is a ZKP-based payment channel protocol.
# Replace with actual Plasma-specific configuration when available.
[Node]
DataDir = "/data"
`

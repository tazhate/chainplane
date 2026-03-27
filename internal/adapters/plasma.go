package adapters

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// NOTE: Plasma Next (plasma.to) is a ZKP-based payment channel protocol, not a
// traditional full-node chain. This adapter is a placeholder; correct
// implementation requires Plasma-specific client.

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// plasma-next/node:v0.1.0 is a placeholder image — no public node image exists yet.
const defaultPlasmaImage = "plasma-next/node:v0.1.0"

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

func (a *plasmaAdapter) DefaultImage(_ string) string {
	return defaultPlasmaImage
}

func (a *plasmaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", plasmaConfig, nil
}

func (a *plasmaAdapter) HealthCheck(_ context.Context, _ string) (SyncStatus, error) {
	return SyncStatus{}, fmt.Errorf("plasma adapter not implemented")
}

func (a *plasmaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
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

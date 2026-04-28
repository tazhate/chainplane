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

// NOTE: Monad node access is permissioned. No public Docker image confirmed as of 2026.
const defaultMonadImage = "monadlabs/monad-node:latest"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type monadAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainMonad, &monadAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const monadConfig = `{
  "rpc": {
    "http": true,
    "http_addr": "0.0.0.0",
    "http_port": 8545,
    "ws": true,
    "ws_addr": "0.0.0.0",
    "ws_port": 8546
  },
  "p2p": {
    "port": 30303
  },
  "data_dir": "/data"
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *monadAdapter) DefaultImage(_ string) string {
	return defaultMonadImage
}

func (a *monadAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "monad.json", monadConfig, nil
}

// HealthCheck uses the standard EVM health check (eth_syncing) as Monad is EVM-compatible.
func (a *monadAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *monadAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *monadAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *monadAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

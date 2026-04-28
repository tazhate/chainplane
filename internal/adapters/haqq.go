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

const defaultHaqqImage = "alhaqq/haqq:v1.8.1"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type haqqAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainHaqq, &haqqAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *haqqAdapter) DefaultImage(_ string) string {
	return defaultHaqqImage
}

func (a *haqqAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", haqqConfig, nil
}

func (a *haqqAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *haqqAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return append(evmPorts(26656), corev1.ContainerPort{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP})
}

func (a *haqqAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "26660"}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const haqqConfig = `# Haqq Network (Islamic Coin) — Cosmos EVM chain node configuration
[json-rpc]
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
api = "eth,net,web3,txpool"
enable = true

[p2p]
laddr = "tcp://0.0.0.0:26656"
max_num_inbound_peers = 40
max_num_outbound_peers = 10

[statesync]
enable = false
`

func (a *haqqAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *haqqAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "alhaqq/haqq",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

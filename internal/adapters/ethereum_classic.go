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

// NOTE: Besu has deprecated ETC/classic network support. Consider migrating to etclabscore/core-geth.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type ethereumClassicAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainEthereumClassic, &ethereumClassicAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *ethereumClassicAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainEthereumClassic, client)
}

func (a *ethereumClassicAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", ethereumClassicConfig, nil
}

func (a *ethereumClassicAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *ethereumClassicAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *ethereumClassicAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *ethereumClassicAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("1000Gi"),
	}
}

func (a *ethereumClassicAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "hyperledger/besu",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const ethereumClassicConfig = `# Ethereum Classic (Besu) node configuration
network="classic"
data-path="/data"
rpc-http-enabled=true
rpc-http-host="0.0.0.0"
rpc-http-port=8545
rpc-http-cors-origins=["*"]
rpc-http-api=["ETH","NET","WEB3","TXPOOL","DEBUG"]
rpc-ws-enabled=true
rpc-ws-host="0.0.0.0"
rpc-ws-port=8546
host-allowlist=["*"]
sync-mode="SNAP"
p2p-port=30303
max-peers=50
`

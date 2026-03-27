package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// NOTE: Besu has deprecated ETC/classic network support. Consider migrating to etclabscore/core-geth.
const defaultEthereumClassicImage = "hyperledger/besu:24.12.0"

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

func (a *ethereumClassicAdapter) DefaultImage(_ string) string {
	return defaultEthereumClassicImage
}

func (a *ethereumClassicAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", ethereumClassicConfig, nil
}

func (a *ethereumClassicAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *ethereumClassicAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
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

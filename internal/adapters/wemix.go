package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultWemixImage = "wemixnetwork/wemix:v1.2.0"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type wemixAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainWemix, &wemixAdapter{
		baseAdapter: baseAdapter{livenessPort: 8588},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *wemixAdapter) DefaultImage(_ string) string {
	return defaultWemixImage
}

func (a *wemixAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", wemixConfig, nil
}

func (a *wemixAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *wemixAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "http", ContainerPort: 8588, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8598, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 8589, Protocol: corev1.ProtocolTCP},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const wemixConfig = `# Wemix EVM-compatible chain node configuration
[Eth]
SyncMode = "snap"
NetworkId = 1111

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8588
HTTPVirtualHosts = ["*"]
HTTPCorsDomain = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool"]
WSHost = "0.0.0.0"
WSPort = 8598
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":8589"
`

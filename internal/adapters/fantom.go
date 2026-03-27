package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultFantomImage = "fantomfoundation/go-opera:v1.1.3-txtracing"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type fantomAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainFantom, &fantomAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *fantomAdapter) DefaultImage(_ string) string {
	return defaultFantomImage
}

func (a *fantomAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", fantomConfig, nil
}

func (a *fantomAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *fantomAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *fantomAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(5050)
}

func (a *fantomAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"--datadir", "/data",
		"--http",
		"--http.addr", "0.0.0.0",
		"--http.port", "8545",
		"--http.api", "eth,net,web3,ftm",
		"--http.vhosts", "*",
		"--http.corsdomain", "*",
		"--ws",
		"--ws.addr", "0.0.0.0",
		"--ws.port", "8546",
		"--ws.api", "eth,net,web3",
		"--ws.origins", "*",
		"--port", "5050",
		"--maxpeers", "50",
		"--cache", "4096",
	}
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const fantomConfig = `# Fantom Opera mainnet RPC node
[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "ftm"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":5050"
`

package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultMantleImage = "mantlenetworkio/op-geth:v1.0.3"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type mantleAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainMantle, &mantleAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *mantleAdapter) DefaultImage(_ string) string {
	return defaultMantleImage
}

func (a *mantleAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", mantleConfig, nil
}

func (a *mantleAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *mantleAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *mantleAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

func (a *mantleAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"--datadir", "/data",
		"--http",
		"--http.addr", "0.0.0.0",
		"--http.port", "8545",
		"--http.api", "eth,net,web3,rollup",
		"--http.vhosts", "*",
		"--http.corsdomain", "*",
		"--ws",
		"--ws.addr", "0.0.0.0",
		"--ws.port", "8546",
		"--ws.api", "eth,net,web3",
		"--ws.origins", "*",
		"--port", "30303",
		"--maxpeers", "50",
		"--rollup.disabletxpoolgossip",
	}
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const mantleConfig = `[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "rollup"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

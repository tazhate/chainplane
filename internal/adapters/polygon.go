package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultPolygonImage = "0xpolygon/bor:2.6.3"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type polygonAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainPolygon, &polygonAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string)
// --------------------------------------------------------------------------

const polygonConfig = `# Bor 2.x mainnet RPC node with Heimdall sidecar
chain = "mainnet"
datadir = "/data"
syncmode = "full"
gcmode = "archive"

[jsonrpc]
  [jsonrpc.http]
    enabled = true
    host = "0.0.0.0"
    port = 8545
    api = ["eth", "net", "web3", "txpool", "bor"]
    vhosts = ["*"]
    corsdomain = ["*"]
  [jsonrpc.ws]
    enabled = true
    host = "0.0.0.0"
    port = 8546
    api = ["eth", "net", "web3"]
    origins = ["*"]

[p2p]
  maxpeers = 50

[txpool]
  pricelimit = 25000000000

[bor]
  withoutheimdall = false

[heimdall]
  url = "http://localhost:1317"

[log]
  verbosity = 3
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *polygonAdapter) DefaultImage(_ string) string {
	return defaultPolygonImage
}

func (a *polygonAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", polygonConfig, nil
}

// ContainerCommand overrides entrypoint to use Bor 2.x server subcommand.
func (a *polygonAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"bor", "server", "--config", "/config/config.toml"}
}

func (a *polygonAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *polygonAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *polygonAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30303, HostPort: 30303, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30303, HostPort: 30303, Protocol: corev1.ProtocolUDP},
	}
}

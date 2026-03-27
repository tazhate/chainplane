package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// NOTE: fantomfoundation/sonic:v1.2.0 was the legacy Fantom Opera client and is
// explicitly INCOMPATIBLE with Sonic mainnet. Sonic is a standalone L1 chain.
const defaultSonicImage = "ghcr.io/0xsoniclabs/sonic:v2.1.6"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type sonicAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainSonic, &sonicAdapter{
		baseAdapter: baseAdapter{livenessPort: 18545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *sonicAdapter) DefaultImage(_ string) string {
	return defaultSonicImage
}

func (a *sonicAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", sonicConfig, nil
}

func (a *sonicAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *sonicAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 18545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 18546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 5050, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 5050, Protocol: corev1.ProtocolUDP},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const sonicConfig = `# Sonic (Fantom successor) mainnet RPC node
[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 18545
HTTPModules = ["eth", "net", "web3", "ftm", "sonic"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 18546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":5050"
`

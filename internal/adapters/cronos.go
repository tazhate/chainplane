package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultCronosImage = "crypto-org-chain/cronos:v1.4.4"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type cronosAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainCronos, &cronosAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *cronosAdapter) DefaultImage(_ string) string {
	return defaultCronosImage
}

func (a *cronosAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", cronosConfig, nil
}

func (a *cronosAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *cronosAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 26656, Protocol: corev1.ProtocolUDP},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const cronosConfig = `# Cronos mainnet RPC node
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

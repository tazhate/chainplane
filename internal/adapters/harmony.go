package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultHarmonyImage = "harmonyone/harmony:v8.5.4"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type harmonyAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainHarmony, &harmonyAdapter{
		baseAdapter: baseAdapter{livenessPort: 9500},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *harmonyAdapter) DefaultImage(_ string) string {
	return defaultHarmonyImage
}

func (a *harmonyAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "harmony.conf", harmonyConfig, nil
}

func (a *harmonyAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *harmonyAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9500, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9800, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 9000, Protocol: corev1.ProtocolUDP},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const harmonyConfig = `# Harmony ONE mainnet RPC node
Version = "2.5.13"

[General]
DataDir = "/data"
EnablePruning = false
IsArchival = false
NoStaking = true
NodeType = "explorer"
ShardID = 0

[HTTP]
AuthPort = 9501
Enabled = true
IP = "0.0.0.0"
Port = 9500
RosettaEnabled = false
RosettaPort = 9700

[WS]
AuthPort = 9801
Enabled = true
IP = "0.0.0.0"
Port = 9800

[P2P]
IP = "0.0.0.0"
KeyFile = "/data/.hmykey"
Port = 9000
`

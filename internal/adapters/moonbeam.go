package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultMoonbeamImage = "moonbeamfoundation/moonbeam:v0.39.1"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type moonbeamAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainMoonbeam, &moonbeamAdapter{
		baseAdapter: baseAdapter{livenessPort: 9944},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *moonbeamAdapter) DefaultImage(_ string) string {
	return defaultMoonbeamImage
}

func (a *moonbeamAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "moonbeam.json", moonbeamConfig, nil
}

func (a *moonbeamAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const moonbeamConfig = `{
  "name": "moonbeam-node",
  "rpc": {
    "port": 9944,
    "external": true,
    "cors": "all",
    "methods": "unsafe"
  },
  "ws": {
    "port": 9945,
    "external": true
  },
  "network": {
    "port": 30333,
    "nodeKey": ""
  },
  "base": {
    "path": "/data",
    "chain": "moonbeam"
  }
}`

// ContainerArgs passes --base-path /data and the config file so the node uses the PVC mount.
func (a *moonbeamAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"--base-path", "/data",
		"--config", "/config/moonbeam.json",
	}
}

func (a *moonbeamAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9944, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9945, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30333, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30333, Protocol: corev1.ProtocolUDP},
	}
}

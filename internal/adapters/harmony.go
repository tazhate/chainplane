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

func (a *harmonyAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainHarmony, client)
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
		{Name: "metrics", ContainerPort: 9900, Protocol: corev1.ProtocolTCP},
	}
}

func (a *harmonyAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *harmonyAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "harmonyone/harmony",
		TagPattern: `^v\d+\.\d+\.\d+$`,
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

[Prometheus]
Enabled = true
IP = "0.0.0.0"
Port = 9900
EnablePush = false
`

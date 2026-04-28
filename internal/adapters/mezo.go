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

// Mezo is Cosmos SDK with EVMOS EVM extension (not OP Stack).
const defaultMezoImage = "mezo/mezod:v2.0.1"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type mezoAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainMezo, &mezoAdapter{
		baseAdapter: baseAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *mezoAdapter) DefaultImage(_ string) string {
	return defaultMezoImage
}

func (a *mezoAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", mezoConfig, nil
}

// ContainerArgs passes --home /data so mezod reads from the PVC mount.
func (a *mezoAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"start", "--home", "/data"}
}

func (a *mezoAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "api", ContainerPort: 1317, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "evm-rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "evm-ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck uses evmHealthCheck since mezod exposes EVM JSON-RPC at 8545.
func (a *mezoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *mezoAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *mezoAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "mezo/mezod",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const mezoConfig = `# Mezo node config (Cosmos SDK + EVMOS EVM extension)
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = false

[telemetry]
enabled = true
prometheus-retention-time = 60

[json-rpc]
enable = true
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
`

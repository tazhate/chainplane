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
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *sonicAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *sonicAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *sonicAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "0xsoniclabs/sonic",
		TagPattern: `^v\d+\.\d+\.\d+$`,
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

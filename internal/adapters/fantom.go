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
	return append(evmPorts(5050), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
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
		"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060",
	}
}

func (a *fantomAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

func (a *fantomAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "fantomfoundation/go-opera",
		TagPattern: `^v\d+\.\d+\.\d+`,
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

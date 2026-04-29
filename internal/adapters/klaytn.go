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

type klaytnAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainKlaytn, &klaytnAdapter{
		baseAdapter: baseAdapter{livenessPort: 8551},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *klaytnAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainKlaytn, client)
}

func (a *klaytnAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "kaia.toml", klaytnConfig, nil
}

func (a *klaytnAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *klaytnAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8551, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8552, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 32323, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 32323, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *klaytnAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"--metrics",
		"--metrics.addr=0.0.0.0",
		"--metrics.port=6060",
	}
}

func (a *klaytnAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *klaytnAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "klaytn/klaytn",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const klaytnConfig = `# Kaia (Klaytn) mainnet EN (Endpoint Node) configuration
[node]
datadir = "/data"

[rpc]
http.addr = "0.0.0.0"
http.port = 8551
http.api = ["eth", "net", "web3", "kaia", "debug"]
http.vhosts = ["*"]
http.corsdomain = ["*"]
ws.addr = "0.0.0.0"
ws.port = 8552
ws.api = ["eth", "net", "web3", "kaia"]
ws.origins = ["*"]

[p2p]
port = 32323
maxpeers = 25
`

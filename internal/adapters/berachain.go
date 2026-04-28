package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// NOTE: Berachain uses a dual-client architecture (BeaconKit CL + reth EL).
// This adapter manages only the BeaconKit consensus layer.
// A companion reth execution-layer pod MUST run separately and be reachable
// via Engine API at port 8551. The health check targets port 8545 which is
// exposed by the EL (reth), not this container.
//
// Official images:
//
//	CL (this adapter): ghcr.io/berachain/beacon-kit
//	EL (companion):    ghcr.io/berachain/reth
//
// See: https://docs.berachain.com/nodes/run-a-node
const defaultBerachainImage = "ghcr.io/berachain/beacon-kit:v0.2.0"

type berachainAdapter struct {
	baseAdapter
}

func init() {
	Register(nodesv1alpha1.ChainBerachain, &berachainAdapter{
		// livenessPort targets the EL reth JSON-RPC (must be co-deployed)
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

func (a *berachainAdapter) DefaultImage(_ string) string {
	return defaultBerachainImage
}

func (a *berachainAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", berachainConfig, nil
}

func (a *berachainAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *berachainAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		// CometBFT P2P
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		// CometBFT RPC
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		// Engine API (connects to EL)
		{Name: "engine", ContainerPort: 8551, Protocol: corev1.ProtocolTCP},
		// Cosmos SDK telemetry (Prometheus)
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

func (a *berachainAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"start", "--home", "/data"}
}

func (a *berachainAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *berachainAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "berachain/beacon-kit",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

const berachainConfig = `# BeaconKit consensus layer — Berachain mainnet
# Requires co-deployed reth EL reachable at http://127.0.0.1:8551

[beacon-kit]
  [beacon-kit.engine]
    # Engine API endpoint of the co-deployed reth execution layer
    rpc-dial-url = "http://127.0.0.1:8551"
    jwt-secret-path = "/jwt.hex"

  [beacon-kit.node-api]
    enabled = true
    address = "0.0.0.0"
    port = 3500

[telemetry]
  enabled = true
  prometheus-retention-time = 60
  prometheus-addr = "0.0.0.0:26660"

[comet]
  [comet.p2p]
    laddr = "tcp://0.0.0.0:26656"
    max-num-inbound-peers = 40
    max-num-outbound-peers = 10

  [comet.rpc]
    laddr = "tcp://0.0.0.0:26657"
`

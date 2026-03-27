package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// Hemi is an OP Stack L2 with Bitcoin finality guarantees.
// The EVM execution layer is hemilabs/op-geth (fork of op-geth).
// NOTE: Do NOT use hemilabs/heminetwork — that image contains only the
// Bitcoin-finality daemon suite (bfgd/bssd/popmd), not the EVM node.
const defaultHemiImage = "hemilabs/op-geth:v1.101408.0"

const defaultHemiL1URL = "http://ethereum-mainnet-01.blockchain-nodes.svc.cluster.local:8545"

type hemiAdapter struct {
	baseAdapter
}

func init() {
	Register(nodesv1alpha1.ChainHemi, &hemiAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

func (a *hemiAdapter) DefaultImage(_ string) string {
	return defaultHemiImage
}

func (a *hemiAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.toml", hemiConfig, nil
}

func (a *hemiAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *hemiAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}

func (a *hemiAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--config", "/config/config.toml"}
}

// ContainerEnv provides the L1 RPC endpoint required by OP Stack op-geth.
func (a *hemiAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "OP_GETH_GENESIS_FILE_PATH", Value: "/config/genesis.json"},
		{Name: "L1_RPC_URL", Value: defaultHemiL1URL},
	}
}

const hemiConfig = `# Hemi op-geth configuration (OP Stack L2)
[Eth]
NetworkId = 43111
SyncMode = "snap"

[Node]
DataDir = "/data"

[Node.HTTPHost]
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPVirtualHosts = ["*"]
HTTPCorsDomain = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool", "engine"]

[Node.WSHost]
WSHost = "0.0.0.0"
WSPort = 8546
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.AuthAddr]
AuthAddr = "0.0.0.0"
AuthPort = 8551
JWTSecret = "/jwt.hex"

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

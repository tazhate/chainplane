package adapters

import (
	"bytes"
	"context"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultBSCImage = "ghcr.io/bnb-chain/bsc:1.6.7"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type bscAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainBSC, &bscAdapter{
		baseAdapter: baseAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var bscConfigTpl = template.Must(template.New("bsc-config").Parse(`[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50

[Eth]
NetworkId = {{ .NetworkID }}
SyncMode = "snap"

[Eth.TxPool]
PriceLimit = 3000000000
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *bscAdapter) DefaultImage(_ string) string {
	return defaultBSCImage
}

func (a *bscAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	networkID := 56
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		networkID = 97
	}
	var buf bytes.Buffer
	if err := bscConfigTpl.Execute(&buf, struct{ NetworkID int }{networkID}); err != nil {
		return "", "", err
	}
	return "config.toml", buf.String(), nil
}

// ContainerCommand downloads BSC genesis.json on first run (not bundled in image),
// then starts geth. HTTP/WS flags override config file to ensure they take effect.
func (a *bscAdapter) ContainerCommand(spec nodesv1alpha1.BlockchainNodeSpec) []string {
	genesisURL := "https://raw.githubusercontent.com/bnb-chain/bsc/master/core/genesis/genesis.json"
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		genesisURL = "https://raw.githubusercontent.com/bnb-chain/bsc/master/core/genesis/testnet.json"
	}
	return []string{
		"sh", "-c",
		`if [ ! -d /data/geth ]; then wget -q -O /tmp/genesis.json '` + genesisURL + `' && geth --datadir /data init /tmp/genesis.json; fi; exec geth --config /config/config.toml --datadir /data --cache 4096 --history.transactions 0 --http --http.addr 0.0.0.0 --http.port 8545 --http.api eth,net,web3,txpool --http.vhosts '*' --http.corsdomain '*' --ws --ws.addr 0.0.0.0 --ws.port 8546 --ws.api eth,net,web3`,
	}
}

func (a *bscAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *bscAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *bscAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30311)
}

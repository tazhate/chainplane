package adapters

import (
	"bytes"
	"context"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
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
// then starts geth. Uses set -e with explicit error checks so a failed or partial
// genesis download causes an exit 1 (pod restart) rather than silent proceeding.
func (a *bscAdapter) ContainerCommand(spec nodesv1alpha1.BlockchainNodeSpec) []string {
	genesisURL := "https://raw.githubusercontent.com/bnb-chain/bsc/master/core/genesis/genesis.json"
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		genesisURL = "https://raw.githubusercontent.com/bnb-chain/bsc/master/core/genesis/testnet.json"
	}
	script := `set -e
if [ ! -d /data/geth ]; then
  echo "[bsc] Downloading genesis.json..."
  wget -q -O /tmp/genesis.json '` + genesisURL + `' || {
    rm -f /tmp/genesis.json
    echo "[bsc] genesis.json download FAILED — exiting for retry"
    exit 1
  }
  [ -s /tmp/genesis.json ] || { rm -f /tmp/genesis.json; echo "[bsc] genesis.json empty — exiting for retry"; exit 1; }
  geth --datadir /data init /tmp/genesis.json || { echo "[bsc] geth init FAILED"; exit 1; }
  rm -f /tmp/genesis.json
fi
exec geth --config /config/config.toml --datadir /data --datadir.ancient /data/geth/chaindata/ancient --syncmode full --tries-verify-mode none --state.scheme=path --db.engine=pebble --cache 8000 --history.transactions 0 --http --http.addr 0.0.0.0 --http.port 8545 --http.api eth,net,web3,txpool --http.vhosts '*' --http.corsdomain '*' --ws --ws.addr 0.0.0.0 --ws.port 8546 --ws.api eth,net,web3`
	return []string{"sh", "-c", script}
}

func (a *bscAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// StartupProbe gives BSC up to 30min for PBSS chaindata recovery on large
// (1+ TB) snapshots. Without it, the LivenessProbe kills geth at the 7-minute
// mark before pebble DB finishes opening and binds :8545.
func (a *bscAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 60, 30, 10, 60)
}

func (a *bscAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(8545, 60, 30, 10, 5)
}

func (a *bscAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return append(evmPorts(30311), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *bscAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *bscAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "bnb-chain/bsc",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

func (a *bscAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

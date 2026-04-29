/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package adapters

import (
	"bytes"
	"context"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type bscAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainBSC, &bscAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
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

func (a *bscAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainBSC, client)
}

func (a *bscAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	networkID := 56
	if spec.Network == chainsv1alpha2.NetworkTestnet {
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
func (a *bscAdapter) ContainerCommand(spec chainsv1alpha2.ChainInstanceSpec) []string {
	genesisURL := "https://raw.githubusercontent.com/bnb-chain/bsc/master/core/genesis/genesis.json"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
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
func (a *bscAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(8545, 60, 30, 10, 60)
}

func (a *bscAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(8545, 60, 30, 10, 5)
}

func (a *bscAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30311), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *bscAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
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

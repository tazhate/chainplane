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

const defaultBitcoinImage = "lncm/bitcoind:v28.0"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type bitcoinAdapter struct {
	utxoAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainBitcoin, &bitcoinAdapter{
		utxoAdapter: utxoAdapter{
			baseAdapter:    baseAdapter{livenessPort: 8332},
			rpcUserEnv:     "BTC_RPC_USER",
			rpcPasswordEnv: "BTC_RPC_PASSWORD",
			defaultUser:    "rpc",
			defaultPass:    "rpc",
			useRetry:       true,
		},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var btcConfigTpl = template.Must(template.New("bitcoin.conf").Parse(`# bitcoin.conf
{{- if .IsTestnet }}
testnet=1
datadir=/data
txindex=1

[test]
server=1
rpcallowip=10.42.0.0/16
rpcbind=0.0.0.0
rpcport=18332
rpcuser={{ .RPCUser }}
rpcpassword={{ .RPCPassword }}
{{- else }}
server=1
rpcallowip=10.42.0.0/16
rpcbind=0.0.0.0
rpcport=8332
rpcuser={{ .RPCUser }}
rpcpassword={{ .RPCPassword }}
testnet=0
datadir=/data
txindex=1
{{- end }}
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *bitcoinAdapter) DefaultImage(_ string) string {
	return defaultBitcoinImage
}

func (a *bitcoinAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	user, pass := a.rpcCredentials()
	var buf bytes.Buffer
	err := btcConfigTpl.Execute(&buf, struct {
		IsTestnet   bool
		RPCUser     string
		RPCPassword string
	}{
		IsTestnet:   spec.Network == nodesv1alpha1.NetworkTestnet,
		RPCUser:     user,
		RPCPassword: pass,
	})
	if err != nil {
		return "", "", err
	}
	return "bitcoin.conf", buf.String(), nil
}

func (a *bitcoinAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return utxoHealthCheck(ctx, rpcURL, &a.utxoAdapter, "synced-exempt")
}

func (a *bitcoinAdapter) LivenessProbe(spec nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	port := int32(8332)
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		port = 18332
	}
	return tcpProbe(port, 30, 30, 10, 3)
}

// ContainerArgs passes the config file path explicitly.
// lncm/bitcoind ENTRYPOINT runs "exec bitcoind $@" — without -conf, bitcoind
// looks for config at $BITCOIN_DATA/bitcoin.conf (~/.bitcoin/) and never reads
// the ConfigMap mounted at /config/bitcoin.conf.
func (a *bitcoinAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"-conf=/config/bitcoin.conf"}
}

func (a *bitcoinAdapter) ContainerPorts(spec nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		return []corev1.ContainerPort{
			{Name: "rpc", ContainerPort: 18332, Protocol: corev1.ProtocolTCP},
			{Name: "p2p", ContainerPort: 18333, Protocol: corev1.ProtocolTCP},
		}
	}
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8332, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 8333, Protocol: corev1.ProtocolTCP},
	}
}

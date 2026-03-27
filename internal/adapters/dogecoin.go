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

// NOTE: No official Dogecoin Docker image exists. Using community image ruimarinho/dogecoin.
const defaultDogecoinImage = "ruimarinho/dogecoin:1-alpine"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type dogecoinAdapter struct {
	utxoAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainDogecoin, &dogecoinAdapter{
		utxoAdapter: utxoAdapter{
			baseAdapter:    baseAdapter{livenessPort: 22555},
			rpcUserEnv:     "DOGE_RPC_USER",
			rpcPasswordEnv: "DOGE_RPC_PASS",
			defaultUser:    "rpc",
			defaultPass:    "rpc",
			useRetry:       false,
		},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var dogeConfigTpl = template.Must(template.New("dogecoin.conf").Parse(`server=1
rpcallowip=10.42.0.0/16
rpcbind=0.0.0.0
rpcport=22555
rpcuser={{ .RPCUser }}
rpcpassword={{ .RPCPassword }}
testnet={{ .Testnet }}
datadir=/data
txindex=1
maxconnections=125
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *dogecoinAdapter) DefaultImage(_ string) string {
	return defaultDogecoinImage
}

func (a *dogecoinAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	testnet, user, pass := utxoConfigValues(spec, &a.utxoAdapter)
	var buf bytes.Buffer
	err := dogeConfigTpl.Execute(&buf, struct {
		Testnet     string
		RPCUser     string
		RPCPassword string
	}{
		Testnet:     testnet,
		RPCUser:     user,
		RPCPassword: pass,
	})
	if err != nil {
		return "", "", err
	}
	return "dogecoin.conf", buf.String(), nil
}

func (a *dogecoinAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return utxoHealthCheck(ctx, rpcURL, &a.utxoAdapter, "synced-exempt")
}

func (a *dogecoinAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 22555, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 22556, Protocol: corev1.ProtocolTCP},
	}
}

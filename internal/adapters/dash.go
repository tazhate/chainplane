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

// Live production nodes confirmed on dashpay/dashd:23.1.0 (without v-prefix).
const defaultDashImage = "dashpay/dashd:23.1.0"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type dashAdapter struct {
	utxoAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainDash, &dashAdapter{
		utxoAdapter: utxoAdapter{
			baseAdapter:    baseAdapter{livenessPort: 9998},
			rpcUserEnv:     "DASH_RPC_USER",
			rpcPasswordEnv: "DASH_RPC_PASSWORD",
			defaultUser:    "rpc",
			defaultPass:    "rpc",
			useRetry:       false,
		},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var dashConfigTpl = template.Must(template.New("dash.conf").Parse(`server=1
rpcallowip=10.42.0.0/16
rpcbind=0.0.0.0
rpcport=9998
rpcuser={{ .RPCUser }}
rpcpassword={{ .RPCPassword }}
testnet={{ .Testnet }}
datadir=/data
txindex=1
dbcache=1024
maxconnections=125
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *dashAdapter) DefaultImage(_ string) string {
	return defaultDashImage
}

func (a *dashAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	testnet, user, pass := utxoConfigValues(spec, &a.utxoAdapter)
	var buf bytes.Buffer
	err := dashConfigTpl.Execute(&buf, struct {
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
	return "dash.conf", buf.String(), nil
}

func (a *dashAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return utxoHealthCheck(ctx, rpcURL, &a.utxoAdapter, "synced-exempt")
}

// ContainerArgs passes the config file path explicitly.
// dashpay/dashd ENTRYPOINT is "docker-entrypoint.sh"; the first arg "dashd"
// triggers the entrypoint to exec dashd with remaining args.
// Without -conf, dashd looks for dash.conf in the default datadir and never
// reads the ConfigMap mounted at /config/dash.conf.
func (a *dashAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"dashd", "-conf=/config/dash.conf", "-printtoconsole"}
}

func (a *dashAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9998, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9999, Protocol: corev1.ProtocolTCP},
	}
}

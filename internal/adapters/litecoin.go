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

// uphold/docker-litecoin-core:0.21 contains v0.21.2.2 (latest published, Feb 2024).
// Litecoin Core v0.21.4 (security fixes CVE-2024-35202) is not yet available on
// any major Docker Hub publisher as of 2026-03. Track: https://github.com/uphold/docker-litecoin-core
const defaultLitecoinImage = "uphold/litecoin-core:0.21"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type litecoinAdapter struct {
	utxoAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainLitecoin, &litecoinAdapter{
		utxoAdapter: utxoAdapter{
			baseAdapter:    baseAdapter{livenessPort: 9332},
			rpcUserEnv:     "LTC_RPC_USER",
			rpcPasswordEnv: "LTC_RPC_PASSWORD",
			defaultUser:    "rpc",
			defaultPass:    "rpc",
			useRetry:       true,
		},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var ltcConfigTpl = template.Must(template.New("litecoin.conf").Parse(`server=1
rpcallowip=10.42.0.0/16
rpcbind=0.0.0.0
rpcport=9332
rpcuser={{ .RPCUser }}
rpcpassword={{ .RPCPassword }}
rpcworkqueue=128
rpcthreads=8
testnet={{ .Testnet }}
datadir=/data
txindex=1
dbcache=4096
maxconnections=125
par=4
maxorphantx=10
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *litecoinAdapter) DefaultImage(_ string) string {
	return defaultLitecoinImage
}

func (a *litecoinAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	testnet, user, pass := utxoConfigValues(spec, &a.utxoAdapter)
	var buf bytes.Buffer
	err := ltcConfigTpl.Execute(&buf, struct {
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
	return "litecoin.conf", buf.String(), nil
}

func (a *litecoinAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return utxoHealthCheck(ctx, rpcURL, &a.utxoAdapter, "ibd-exempt")
}

func (a *litecoinAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9332, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9333, Protocol: corev1.ProtocolTCP},
	}
}

// ContainerArgs passes the config file path explicitly.
// uphold/litecoin-core ENTRYPOINT runs "exec litecoind -datadir=$LITECOIN_DATA $@".
// With LITECOIN_DATA=/data the node looks for litecoin.conf at /data/litecoin.conf,
// NOT at /config/litecoin.conf (the ConfigMap mount path).
func (a *litecoinAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"-conf=/config/litecoin.conf"}
}

// ContainerEnv overrides LITECOIN_DATA so the image entrypoint uses /data (PVC)
// instead of the default /home/litecoin/.litecoin (ephemeral overlay).
func (a *litecoinAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "LITECOIN_DATA", Value: "/data"},
	}
}

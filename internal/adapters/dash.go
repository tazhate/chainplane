package adapters

import (
	"bytes"
	"context"
	"strconv"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// Live production nodes confirmed on dashpay/dashd:23.1.0 (without v-prefix).

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
rpcallowip=0.0.0.0/0
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

func (a *dashAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainDash, client)
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
// Per-node flags (e.g. -reindex) go in CRD .spec.extraArgs — they are prepended
// by buildContainerArgs, so DO NOT duplicate -conf or -printtoconsole there.
func (a *dashAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"dashd", "-conf=/config/dash.conf"}
}

func (a *dashAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9998, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9999, Protocol: corev1.ProtocolTCP},
	}
}

// Sidecars returns a bitcoin-prometheus-exporter sidecar (compatible with Dash).
// Credentials are read from the same env vars used by the node itself.
func (a *dashAdapter) Sidecars(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.Container {
	user, pass := a.rpcCredentials()
	if user == "" {
		return nil
	}
	return []corev1.Container{
		{
			Name:  "metrics-exporter",
			Image: "jvstein/bitcoin-prometheus-exporter:v0.8.0",
			Ports: []corev1.ContainerPort{
				{Name: "metrics", ContainerPort: 9332, Protocol: corev1.ProtocolTCP},
			},
			Env: []corev1.EnvVar{
				{Name: "BITCOIN_RPC_HOST", Value: "localhost"},
				{Name: "BITCOIN_RPC_PORT", Value: strconv.Itoa(9998)},
				{Name: "BITCOIN_RPC_USER", Value: user},
				{Name: "BITCOIN_RPC_PASSWORD", Value: pass},
			},
		},
	}
}

func (a *dashAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "dashpay/dashd",
		TagPattern: `^\d+\.\d+`,
	}
}

func (a *dashAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("2Gi"),
		Storage:       resource.MustParse("50Gi"),
	}
}

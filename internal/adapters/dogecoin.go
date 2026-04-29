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

// NOTE: No official Dogecoin Docker image exists. Using community image ruimarinho/dogecoin.

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
rpcallowip=0.0.0.0/0
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

func (a *dogecoinAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainDogecoin, client)
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
		{Name: "metrics", ContainerPort: 9332, Protocol: corev1.ProtocolTCP},
	}
}

func (a *dogecoinAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "ruimarinho/dogecoin",
		TagPattern: `^\d+-alpine$`,
	}
}

func (a *dogecoinAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

// Sidecars returns a bitcoin-prometheus-exporter sidecar for Dogecoin nodes.
// The exporter connects to the local RPC and exposes Prometheus metrics on port 9332.
func (a *dogecoinAdapter) Sidecars(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.Container {
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
				{Name: "BITCOIN_RPC_PORT", Value: strconv.Itoa(22555)},
				{Name: "BITCOIN_RPC_USER", Value: user},
				{Name: "BITCOIN_RPC_PASSWORD", Value: pass},
			},
		},
	}
}

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
	"strconv"
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

type bitcoinAdapter struct {
	utxoProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainBitcoin, &bitcoinAdapter{
		utxoProtocolAdapter: utxoProtocolAdapter{
			protocolAdapter: protocolAdapter{livenessPort: 8332},
			rpcUserEnv:      "BTC_RPC_USER",
			rpcPasswordEnv:  "BTC_RPC_PASSWORD",
			defaultUser:     "rpc",
			defaultPass:     "rpc",
			useRetry:        true,
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
rpcallowip=0.0.0.0/0
rpcbind=0.0.0.0
rpcport=18332
rpcuser={{ .RPCUser }}
rpcpassword={{ .RPCPassword }}
{{- else }}
server=1
rpcallowip=0.0.0.0/0
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

func (a *bitcoinAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainBitcoin, client)
}

func (a *bitcoinAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	user, pass := a.rpcCredentials()
	var buf bytes.Buffer
	err := btcConfigTpl.Execute(&buf, struct {
		IsTestnet   bool
		RPCUser     string
		RPCPassword string
	}{
		IsTestnet:   spec.Network == chainsv1alpha2.NetworkTestnet,
		RPCUser:     user,
		RPCPassword: pass,
	})
	if err != nil {
		return "", "", err
	}
	return "bitcoin.conf", buf.String(), nil
}

func (a *bitcoinAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return utxoHealthCheck(ctx, rpcURL, &a.utxoProtocolAdapter, "synced-exempt")
}

func (a *bitcoinAdapter) LivenessProbe(spec chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	port := int32(8332)
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		port = 18332
	}
	return tcpProbe(port, 30, 30, 10, 3)
}

// ContainerArgs passes the config file path explicitly.
// lncm/bitcoind ENTRYPOINT runs "exec bitcoind $@" — without -conf, bitcoind
// looks for config at $BITCOIN_DATA/bitcoin.conf (~/.bitcoin/) and never reads
// the ConfigMap mounted at /config/bitcoin.conf.
func (a *bitcoinAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"-conf=/config/bitcoin.conf"}
}

func (a *bitcoinAdapter) ContainerPorts(spec chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	if spec.Network == chainsv1alpha2.NetworkTestnet {
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

// Sidecars returns a bitcoin-prometheus-exporter sidecar for mainnet nodes.
// Credentials are read from the same env vars used by the node itself.
func (a *bitcoinAdapter) Sidecars(spec chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	user, pass := a.rpcCredentials()
	if user == "" {
		return nil
	}
	rpcPort := int32(8332)
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		rpcPort = 18332
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
				{Name: "BITCOIN_RPC_PORT", Value: strconv.Itoa(int(rpcPort))},
				{Name: "BITCOIN_RPC_USER", Value: user},
				{Name: "BITCOIN_RPC_PASSWORD", Value: pass},
			},
		},
	}
}

func (a *bitcoinAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "lncm/bitcoind",
		TagPattern: `^v\d+`,
	}
}

func (a *bitcoinAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

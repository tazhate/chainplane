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

// uphold/docker-litecoin-core:0.21 contains v0.21.2.2 (latest published, Feb 2024).
// Litecoin Core v0.21.4 (security fixes CVE-2024-35202) is not yet available on
// any major Docker Hub publisher as of 2026-03. Track: https://github.com/uphold/docker-litecoin-core

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type litecoinAdapter struct {
	utxoProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainLitecoin, &litecoinAdapter{
		utxoProtocolAdapter: utxoProtocolAdapter{
			protocolAdapter: protocolAdapter{livenessPort: 9332},
			rpcUserEnv:      "LTC_RPC_USER",
			rpcPasswordEnv:  "LTC_RPC_PASSWORD",
			defaultUser:     "rpc",
			defaultPass:     "rpc",
			useRetry:        true,
		},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var ltcConfigTpl = template.Must(template.New("litecoin.conf").Parse(`server=1
rpcallowip=0.0.0.0/0
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

func (a *litecoinAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainLitecoin, client)
}

func (a *litecoinAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	testnet, user, pass := utxoConfigValues(spec, &a.utxoProtocolAdapter)
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
	return utxoHealthCheck(ctx, rpcURL, &a.utxoProtocolAdapter, "ibd-exempt")
}

func (a *litecoinAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9332, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9333, Protocol: corev1.ProtocolTCP},
	}
}

// ContainerArgs passes the config file path explicitly.
// uphold/litecoin-core ENTRYPOINT runs "exec litecoind -datadir=$LITECOIN_DATA $@".
// With LITECOIN_DATA=/data the node looks for litecoin.conf at /data/litecoin.conf,
// NOT at /config/litecoin.conf (the ConfigMap mount path).
func (a *litecoinAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"-conf=/config/litecoin.conf"}
}

// ContainerEnv overrides LITECOIN_DATA so the image entrypoint uses /data (PVC)
// instead of the default /home/litecoin/.litecoin (ephemeral overlay).
func (a *litecoinAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "LITECOIN_DATA", Value: "/data"},
	}
}

// Sidecars returns a bitcoin-prometheus-exporter sidecar (compatible with Litecoin).
// Credentials are read from the same env vars used by the node itself.
func (a *litecoinAdapter) Sidecars(_ chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
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
				{Name: "BITCOIN_RPC_PORT", Value: strconv.Itoa(9332)},
				{Name: "BITCOIN_RPC_USER", Value: user},
				{Name: "BITCOIN_RPC_PASSWORD", Value: pass},
			},
		},
	}
}

func (a *litecoinAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "uphold/litecoin-core",
		TagPattern: `^\d+\.\d+`,
	}
}

func (a *litecoinAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("2Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

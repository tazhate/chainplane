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
			configFile:      "litecoin.conf",
			configTpl:       ltcConfigTpl,
			stallPolicy:     "ibd-exempt",
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
{{- if .RPCAuth }}
rpcauth={{ .RPCAuth }}
{{- end }}
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

// RPCSidecars returns a bitcoin-prometheus-exporter sidecar (compatible with
// Litecoin) that reads the node RPC credentials from secretName.
func (a *litecoinAdapter) RPCSidecars(_ chainsv1alpha2.ChainInstanceSpec, secretName string) []corev1.Container {
	return []corev1.Container{utxoExporterSidecar(9332, secretName)}
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

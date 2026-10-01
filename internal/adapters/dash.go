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

// Live production nodes confirmed on dashpay/dashd:23.1.0 (without v-prefix).

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type dashAdapter struct {
	utxoProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainDash, &dashAdapter{
		utxoProtocolAdapter: utxoProtocolAdapter{
			protocolAdapter: protocolAdapter{livenessPort: 9998},
			rpcUserEnv:      "DASH_RPC_USER",
			rpcPasswordEnv:  "DASH_RPC_PASSWORD",
			configFile:      "dash.conf",
			configTpl:       dashConfigTpl,
			stallPolicy:     "synced-exempt",
			useRetry:        false,
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
{{- if .RPCAuth }}
rpcauth={{ .RPCAuth }}
{{- end }}
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
	return DefaultImageFor(chainsv1alpha2.ChainDash, client)
}

// ContainerArgs passes the config file path explicitly.
// dashpay/dashd ENTRYPOINT is "docker-entrypoint.sh"; the first arg "dashd"
// triggers the entrypoint to exec dashd with remaining args.
// Without -conf, dashd looks for dash.conf in the default datadir and never
// reads the ConfigMap mounted at /config/dash.conf.
// Per-node flags (e.g. -reindex) go in CRD .spec.extraArgs — they are prepended
// by buildContainerArgs, so DO NOT duplicate -conf or -printtoconsole there.
func (a *dashAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"dashd", "-conf=/config/dash.conf"}
}

func (a *dashAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9998, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9999, Protocol: corev1.ProtocolTCP},
	}
}

// RPCSidecars returns a bitcoin-prometheus-exporter sidecar (compatible with
// Dash) that reads the node RPC credentials from secretName.
func (a *dashAdapter) RPCSidecars(_ chainsv1alpha2.ChainInstanceSpec, secretName string) []corev1.Container {
	return []corev1.Container{utxoExporterSidecar(9998, secretName)}
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

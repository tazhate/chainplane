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

// NOTE: No official Dogecoin Docker image exists. Using community image ruimarinho/dogecoin.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type dogecoinAdapter struct {
	utxoProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainDogecoin, &dogecoinAdapter{
		utxoProtocolAdapter: utxoProtocolAdapter{
			protocolAdapter: protocolAdapter{livenessPort: 22555},
			rpcUserEnv:      "DOGE_RPC_USER",
			rpcPasswordEnv:  "DOGE_RPC_PASSWORD",
			configFile:      "dogecoin.conf",
			configTpl:       dogeConfigTpl,
			stallPolicy:     "synced-exempt",
			useRetry:        false,
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
{{- if .RPCAuth }}
rpcauth={{ .RPCAuth }}
{{- end }}
testnet={{ .Testnet }}
datadir=/data
txindex=1
maxconnections=125
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *dogecoinAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainDogecoin, client)
}

func (a *dogecoinAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 22555, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 22556, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9332, Protocol: corev1.ProtocolTCP},
	}
}

func (a *dogecoinAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "fiftysix/dogecoin-core",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

func (a *dogecoinAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

// RPCSidecars returns a bitcoin-prometheus-exporter sidecar for Dogecoin nodes.
// The exporter connects to the local RPC with credentials from secretName and
// exposes Prometheus metrics on port 9332.
func (a *dogecoinAdapter) RPCSidecars(_ chainsv1alpha2.ChainInstanceSpec, secretName string) []corev1.Container {
	return []corev1.Container{utxoExporterSidecar(22555, secretName)}
}

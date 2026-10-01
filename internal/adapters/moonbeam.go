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
	"context"

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

type moonbeamAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMoonbeam, &moonbeamAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 9944},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *moonbeamAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMoonbeam, client)
}

func (a *moonbeamAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "moonbeam.json", moonbeamConfig, nil
}

func (a *moonbeamAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const moonbeamConfig = `{
  "name": "moonbeam-node",
  "rpc": {
    "port": 9944,
    "external": true,
    "cors": "all",
    "methods": "unsafe"
  },
  "prometheus": {
    "port": 9615,
    "external": true
  },
  "network": {
    "port": 30333,
    "nodeKey": ""
  },
  "base": {
    "path": "/data",
    "chain": "moonbeam"
  }
}`

// ContainerArgs selects the chain and sets the RPC, metrics and P2P ports by
// flag: the node has no --config flag, so moonbeam.json only documents them.
func (a *moonbeamAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return moonbeamArgs("moonbeam", spec)
}

// moonbeamArgs returns the parachain flags shared by Moonbeam and Moonriver.
// The embedded relay chain (Polkadot or Kusama, picked by the chain spec)
// keeps its defaults: RPC and metrics on localhost, P2P on 30334. Relay
// flags such as --sync go after a "--" separator in spec.extraArgs.
func moonbeamArgs(chain string, spec chainsv1alpha2.ChainInstanceSpec) []string {
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		chain = "moonbase-alpha"
	}
	return []string{
		"--base-path", "/data",
		"--chain", chain,
		"--name", "k8s-" + chain + "-node",
		"--rpc-port", "9944",
		"--rpc-external",
		"--rpc-cors", "all",
		"--rpc-methods", "unsafe",
		"--prometheus-port", "9615",
		"--prometheus-external",
		"--port", "30333",
	}
}

func (a *moonbeamAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *moonbeamAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "moonbeamfoundation/moonbeam",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *moonbeamAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9944, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30333, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30333, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 9615, Protocol: corev1.ProtocolTCP},
	}
}

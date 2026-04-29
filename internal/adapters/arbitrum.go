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

const defaultArbitrumL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type arbitrumAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainArbitrum, &arbitrumAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *arbitrumAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainArbitrum, client)
}

func (a *arbitrumAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.json", arbitrumConfig, nil
}

func (a *arbitrumAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *arbitrumAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(8547), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 6070, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerArgs injects the --l1.url flag pointing to the L1 Ethereum RPC and enables metrics.
// The L1_RPC_URL env var is set by ContainerEnv and can be overridden via extraEnv.
func (a *arbitrumAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--l1.url=$(L1_RPC_URL)", "--metrics"}
}

// ContainerEnv injects the L1_RPC_URL environment variable required by Arbitrum Nitro.
func (a *arbitrumAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultArbitrumL1URL},
	}
}

func (a *arbitrumAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

func (a *arbitrumAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const arbitrumConfig = `{
  "http": {
    "addr": "0.0.0.0",
    "port": 8545,
    "vhosts": ["*"],
    "corsdomain": ["*"],
    "api": ["eth", "net", "web3", "arb", "debug"]
  },
  "ws": {
    "addr": "0.0.0.0",
    "port": 8546,
    "origins": ["*"],
    "api": ["eth", "net", "web3", "arb"]
  },
  "node": {
    "forwarding-target": "https://arb1.arbitrum.io/rpc",
    "data-dir": "/data"
  },
  "persistent": {
    "chain": "arb1"
  },
  "metrics": true
}
`

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

const defaultTaikoL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type taikoAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainTaiko, &taikoAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *taikoAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainTaiko, client)
}

func (a *taikoAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", taikoConfig, nil
}

func (a *taikoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *taikoAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *taikoAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "us-docker.pkg.dev",
		Repository: "evmchain/images/taiko-geth",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *taikoAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerArgs selects the Taiko genesis built into taiko-geth and passes
// the mounted config, which keeps the data on /data. --taiko picks the
// genesis by network id: 167000 for mainnet, 167013 for the Hoodi testnet.
// Without it the image boots on the Ethereum mainnet genesis under
// /root/.ethereum. The chain advances only when a taiko-client driver feeds
// it blocks derived from L1 over the Engine API, which this adapter does not
// run; on its own geth serves the genesis state.
func (a *taikoAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	networkID := "167000"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		networkID = "167013"
	}
	return append([]string{"--taiko", "--networkid", networkID}, opGethArgs()...)
}

// ContainerEnv injects the L1_RPC_URL environment variable required by Taiko L2 nodes.
func (a *taikoAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultTaikoL1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const taikoConfig = `# Taiko L2 geth node
[Eth]
SyncMode = "snap"

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
`

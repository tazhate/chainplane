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

// immutable-geth needs "geth immutable bootstrap rpc" once to write the zkEVM
// genesis into the data dir; ContainerCommand runs it on an empty volume.

const defaultImmutableZkEVML1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type immutableZkEVMAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainImmutableZkEVM, &immutableZkEVMAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *immutableZkEVMAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainImmutableZkEVM, client)
}

func (a *immutableZkEVMAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", immutableZkEVMConfig, nil
}

func (a *immutableZkEVMAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *immutableZkEVMAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("300Gi"),
	}
}

// No VersionPolicy: immutable-geth publishes only pre-release tags
// (v1.0.0-beta.N), which versioncheck never treats as stable, so there is
// nothing it could track. Bump the beta pin by hand.

func (a *immutableZkEVMAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

// ContainerCommand bootstraps the mainnet genesis on first start, then runs
// geth with the container args.
func (a *immutableZkEVMAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"sh", "-c",
		`[ -d /data/geth/chaindata ] || geth immutable bootstrap rpc --zkevm mainnet --datadir /data; exec geth "$@"`,
		"--"}
}

func (a *immutableZkEVMAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--config", "/config/config.toml",
		"--zkevm", "mainnet",
		"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060",
	}
}

// ContainerEnv injects the L1_RPC_URL environment variable required by Immutable zkEVM (L2).
func (a *immutableZkEVMAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultImmutableZkEVML1URL},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const immutableZkEVMConfig = `# Immutable zkEVM RPC node (geth TOML; unset fields keep the --zkevm defaults)
[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
HTTPModules = ["eth", "net", "web3", "debug", "txpool"]
WSHost = "0.0.0.0"
WSPort = 8546
WSOrigins = ["*"]
WSModules = ["eth", "net", "web3"]

[Node.P2P]
MaxPeers = 50
ListenAddr = ":30303"
`

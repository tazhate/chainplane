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

// victionConfigPath is where the rendered config.toml is mounted.
const victionConfigPath = "/config/config.toml"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type victionAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainViction, &victionAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *victionAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainViction, client)
}

func (a *victionAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", victionConfig, nil
}

func (a *victionAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *victionAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

// ContainerCommand replaces the image entrypoint, which hardcodes the
// relative --datadir data (so chain data landed in /tomochain/data, off the
// volume) and never reads a config file. Like the entrypoint, it writes the
// genesis shipped in /tomochain on first start; a marker file records a
// finished init, so an interrupted one is retried on the next start.
func (a *victionAdapter) ContainerCommand(spec chainsv1alpha2.ChainInstanceSpec) []string {
	genesis := "/tomochain/mainnet.json"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		genesis = "/tomochain/testnet.json"
	}
	script := `set -e
if [ ! -f /data/.genesis-init ]; then
  tomo init --datadir /data ` + genesis + `
  touch /data/.genesis-init
fi
exec tomo "$@"`
	return []string{"sh", "-c", script, "--"}
}

// ContainerArgs passes the mounted config plus the flags the image
// entrypoint sets for a mainnet (88) or testnet (89) RPC node. Node.DataDir
// in the config moves the chain, TomoX and keystore data to /data.
func (a *victionAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	args := []string{"--config", victionConfigPath, "--networkid", "88"}
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		args = []string{"--config", victionConfigPath, "--networkid", "89", "--tomo-testnet"}
	}
	args = append(args, "--gasprice", "250000000", "--targetgaslimit", "30000000")
	return append(args, "--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060")
}

func (a *victionAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *victionAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "buildonviction/node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const victionConfig = `# Viction (formerly TomoChain) EVM-compatible chain node configuration
[Eth]
SyncMode = "full"

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

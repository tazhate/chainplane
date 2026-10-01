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

// Default image: ghcr.io/morph-l2/node (tag in versions_gen.go).

const (
	defaultMorphL1URL       = "http://ethereum:8545"
	defaultMorphL1BeaconURL = "http://ethereum-beacon:5052"
)

// morphNetworkFiles holds the mainnet CometBFT config.toml and genesis.json
// of morphnode, pinned to a run-morph-node commit.
const morphNetworkFiles = "https://raw.githubusercontent.com/morph-l2/run-morph-node/7ee4170ec4b18f80ebb4e4bfd227751219b1e53f/mainnet/node-data/config"

const (
	morphConfigTOMLSHA256 = "567059374d279386e9a7849151b9fd1829f3f75ea98382dfaf2b54b003426a00"
	morphGenesisSHA256    = "f82415910772b9a8843fd90ff94c79d2f8e9c4983d6afffe2e1656d526f53b3e"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type morphAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMorph, &morphAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *morphAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMorph, client)
}

func (a *morphAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", morphConfig, nil
}

func (a *morphAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *morphAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *morphAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "morph-l2/node",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

func (a *morphAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	// morphnode serves CometBFT metrics on :26660 (prometheus = true in the
	// fetched config.toml).
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP})
}

// ContainerCommand runs morphnode, the consensus client the default image
// ships (it has no ENTRYPOINT, so the geth-style --metrics args were exec'd
// as a command). On first start it fetches the mainnet CometBFT
// config.toml and genesis.json from morph-l2/run-morph-node, moved in
// place only when their SHA-256 matches. morphnode drives a co-deployed
// morph-geth through the Engine API on 127.0.0.1:8545/8551 with the JWT at
// /data/jwt-secret.txt (generated on first start when absent) and exits
// without it, so this pod alone does not sync.
func (a *morphAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `set -e
NET=` + morphNetworkFiles + `
C=/data/config
mkdir -p $C /data/data
fetch() {
  [ -f "$C/$1" ] && return 0
  wget -qO "$C/$1.part" "$NET/$1"
  if ! echo "$2  $C/$1.part" | sha256sum -c - >/dev/null 2>&1; then
    rm -f "$C/$1.part"
    echo "$1 from $NET does not match pinned sha256 $2" >&2
    exit 1
  fi
  mv "$C/$1.part" "$C/$1"
}
fetch config.toml ` + morphConfigTOMLSHA256 + `
fetch genesis.json ` + morphGenesisSHA256 + `
exec morphnode --home /data --mainnet --l2.jwt-secret /data/jwt-secret.txt "$@"`
	return []string{"sh", "-c", script, "--"}
}

// ContainerEnv points morphnode at L1 and at the morph-geth Engine API
// (the run-morph-node defaults for a geth next to the node). Contract
// addresses are the run-morph-node mainnet .env values; the beacon RPC is
// required at start.
func (a *morphAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "MORPH_NODE_L2_ETH_RPC", Value: "http://127.0.0.1:8545"},
		{Name: "MORPH_NODE_L2_ENGINE_RPC", Value: "http://127.0.0.1:8551"},
		{Name: "MORPH_NODE_L1_ETH_RPC", Value: defaultMorphL1URL},
		{Name: "MORPH_NODE_L1_ETH_BEACON_RPC", Value: defaultMorphL1BeaconURL},
		{Name: "MORPH_NODE_L1_CHAIN_ID", Value: "1"},
		{Name: "MORPH_NODE_ROLLUP_ADDRESS", Value: "0x759894ced0e6af42c26668076ffa84d02e3cef60"},
		{Name: "MORPH_NODE_SYNC_DEPOSIT_CONTRACT_ADDRESS", Value: "0x3931ade842f5bb8763164bdd81e5361dce6cc1ef"},
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const morphConfig = `# Morph L2 node
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

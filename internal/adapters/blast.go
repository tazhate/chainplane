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

const defaultBlastL1URL = "http://ethereum:8545"

// Blast mainnet genesis from blast-io/deployment, pinned to the commit that
// last changed it and checked by SHA-256 before geth init.
const (
	blastGenesisURL    = "https://raw.githubusercontent.com/blast-io/deployment/197a870f8d9f65d29ad3eef5566f40bb2f5c5353/mainnet/genesis.json"
	blastGenesisSHA256 = "330379e670b42f8a2b9db30b059f6ce7400590b66c70c2d8f7294ba0efeffbae"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type blastAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainBlast, &blastAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *blastAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainBlast, client)
}

func (a *blastAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", blastConfig, nil
}

func (a *blastAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *blastAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerCommand follows blast-io/deployment: blast-geth has no built-in
// Blast network, so on first start it fetches the pinned mainnet genesis.json
// and runs geth init on /data. Without it geth falls back to the Ethereum
// mainnet genesis and exits ("ethash is only supported as a historical
// component of already merged networks"). A marker file records a finished
// init; an interrupted download or init is retried on the next start, and a
// genesis that does not match the pinned SHA-256 exits before init.
func (a *blastAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `set -e
G=/data/genesis.json
if [ ! -f /data/.genesis-init ]; then
  wget -q -O "$G.part" ` + blastGenesisURL + `
  if ! echo "` + blastGenesisSHA256 + `  $G.part" | sha256sum -c - >/dev/null 2>&1; then
    rm -f "$G.part"
    echo "genesis.json does not match pinned sha256 ` + blastGenesisSHA256 + `" >&2
    exit 1
  fi
  mv "$G.part" "$G"
  geth init --datadir /data "$G"
  rm "$G"
  touch /data/.genesis-init
fi
exec geth "$@"`
	return []string{"sh", "-c", script, "--"}
}

// ContainerArgs passes the mounted config and enables the Prometheus metrics
// endpoint on blast-geth.
func (a *blastAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return opGethArgs()
}

// ContainerEnv injects the L1_RPC_URL environment variable required by OP Stack L2 nodes.
func (a *blastAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultBlastL1URL},
	}
}

func (a *blastAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *blastAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "blastio/blast-geth",
		TagPattern: `^mainnet-v\d+`,
		TagPrefix:  "mainnet-",
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// SyncMode is full as in blast-io/deployment, which runs blast-geth with
// discovery off and lets op-node feed it blocks.
const blastConfig = `# Blast L2 (OP Stack) geth node
[Eth]
NetworkId = 81457
SyncMode = "full"

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

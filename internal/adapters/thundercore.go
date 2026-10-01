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

type thundercoreAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainThundercore, &thundercoreAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *thundercoreAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainThundercore, client)
}

func (a *thundercoreAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "override.yaml", thundercoreConfig, nil
}

func (a *thundercoreAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *thundercoreAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 9201, Protocol: corev1.ProtocolTCP})
}

// ContainerCommand runs /pala, the ThunderCore node, the way the upstream
// /entrypoint.sh does (the image ENTRYPOINT is bare tini, so args alone were
// exec'd as a command; pala has no geth-style --metrics flags). The static
// mainnet files of thundercore/public-full are fetched into /data/config and
// re-fetched whenever their SHA-256 differs from the pin; the mounted
// override.yaml (ConfigTemplate) is copied next to them on every start. The
// pins track the image: a release that adds a hardfork needs the matching
// hardfork.yaml.
func (a *thundercoreAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `set -e
NET=` + thundercoreNetworkFiles + `
C=/data/config
mkdir -p $C /data/logs /data/keystore
fetch() {
  wget -qO "$C/$1.part" "$NET/$1"
  if ! echo "$2  $C/$1.part" | sha256sum -c - >/dev/null 2>&1; then
    rm -f "$C/$1.part"
    echo "$1 from $NET does not match pinned sha256 $2" >&2
    exit 1
  fi
  mv "$C/$1.part" "$C/$1"
}
pinned() { [ -f "$C/$1" ] && echo "$2  $C/$1" | sha256sum -c - >/dev/null 2>&1; }
while read -r f sum; do
  pinned $f $sum || fetch $f $sum
done <<EOF
` + thundercoreNetworkFileSums + `EOF
cp /config/override.yaml $C/override.yaml
exec /pala --configPath $C "$@"`
	return []string{"sh", "-c", script, "--"}
}

// InitContainers restores the official chain data snapshot into an empty
// /data/chain, as thundercore/public-full run.sh does: pala cannot sync
// mainnet from an empty data dir (it exits with code 2, "failed to get
// election result from statedb"). The snapshot is a ~320 GB tar.gz streamed
// straight into place; an interrupted restore starts over.
func (a *thundercoreAdapter) InitContainers(spec chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return []corev1.Container{{
		Name:  "thundercore-chaindata",
		Image: a.DefaultImage(spec.Client),
		Command: []string{"sh", "-c", `set -e
[ -d /data/chain/thunder/chaindata ] && exit 0
URL=$(wget -qO- ` + thundercoreChainDataPointer + ` | cut -d , -f 1)
echo "restoring chain data from $URL"
rm -rf /data/chain.part
mkdir -p /data/chain.part
wget -qO- "$URL" | tar -C /data/chain.part -xz
rm -rf /data/chain
mv /data/chain.part /data/chain`},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}}
}

func (a *thundercoreAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *thundercoreAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "thundercore/thunder",
		TagPattern: `^r(?P<version>\d+\.\d+\.\d+)$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// thundercoreNetworkFiles holds the static mainnet config of the official
// full node setup, pinned to a thundercore/public-full commit.
const thundercoreNetworkFiles = "https://raw.githubusercontent.com/thundercore/public-full/0533400f0e751dfbb9d7e235059ca0ec77299686/configs-template/mainnet"

// thundercoreNetworkFileSums lists "file sha256" per line for the files
// under thundercoreNetworkFiles.
const thundercoreNetworkFileSums = `genesis.json e01b85fdbea948784da317650b72124c62a96f3474af7e11d17b6f20ceebc976
genesis_comm_info.json 0e8104f6733b01e7420f823121a312b88b995aa71db988ad69f80ae0e5c2351a
hardfork.yaml 8414d27246e60b94d1f7fa53fb2716f93881519f306af2dce91551525806f19b
r2_comm_info.json bf30f279e7dca0a2c2c80394472919f063031be05511433021939bcf6bfad0dd
thunder.yaml 0bacbdd6c23d9459f66713703ddd9488cd56fe8bb46062da88109f4c432766b6
`

// thundercoreChainDataPointer names the latest mainnet chain data snapshot
// as "<url>,<md5>" (RECOVER_CHAIN_DATA_URL of thundercore/public-full).
const thundercoreChainDataPointer = "https://chaindata-backup-prod-zeus-us-east-1.s3.amazonaws.com/zeus-latest"

// thundercoreConfig is the mainnet override.yaml of thundercore/public-full
// with the container paths moved under /data. pala merges it over
// thunder.yaml, so the log files are redirected here too.
const thundercoreConfig = `loggingId: chainplane
logLevel:
  /: warn
dataDir: /data/chain
logFile: /data/logs/thunder.log
verboseLogFile: /data/logs/thunder.verbose.log
key:
  GenesisCommPath: /data/config/genesis_comm_info.json
  KeyStorePath: /data/keystore
  alterCommPath: /data/config/r2_comm_info.json
pala:
  fromGenesis: false
  bootnode:
    trusted:
      - boot-public.thundercore.com:8888
  isFullNode: true
rpc:
  http:
    hostname: 0.0.0.0
    port: 8545
    modules:
      - eth
      - thunder
      - net
      - web3
  ws:
    hostname: 0.0.0.0
    origins: '*'
    port: 8546
    modules:
      - eth
      - thunder
      - net
      - web3
  maxDelayBlock: 120
  suspendBuffer: 60s
  logs:
    blockRange: -1
  logRequests: True
chain:
  chainID: 108
  genesis: /data/config/genesis.json
  initialSupply: 1E+28
  snapshotCache: 0
accel:
  txpool:
    PriceLimit: 1 # 1 ella
    AccountSlots: 1024
    AccountQueue: 4096
    GlobalSlots: 50000
    GlobalQueue: 10000
    Lifetime: 180s
    EvictionInterval: 12s
  blockmaker:
    TimePerBlock: 1s
    TxPerBlockLimit: -1
eth:
  logFile: /data/logs/thunder.eth.log
  logFilter: "trie=4,state=4"
  txLookupLimit: 0
metrics:
  address: 0.0.0.0:9201
profiling:
  enable: true
  port: 9998
resourceMonitor:
  enable: true
  interval: 10s
`

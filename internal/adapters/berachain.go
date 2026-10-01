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

// NOTE: Berachain uses a dual-client architecture (BeaconKit CL + reth EL).
// This adapter manages only the BeaconKit consensus layer.
// A companion reth execution-layer pod MUST run separately and be reachable
// via Engine API at port 8551. The health check targets port 8545 which is
// exposed by the EL (reth), not this container.
//
// Official images:
//
//	CL (this adapter): ghcr.io/berachain/beacon-kit
//	EL (companion):    ghcr.io/berachain/bera-reth
//
// See: https://docs.berachain.com/nodes/run-a-node

// berachainNetworkFiles is where the mainnet (chain id 80094) genesis, CometBFT
// config and KZG trusted setup are fetched from on first start. Pinned to the
// beacon-kit release of the default image.
const berachainNetworkFiles = "https://raw.githubusercontent.com/berachain/beacon-kit/v1.4.1/testing/networks/80094"

type berachainAdapter struct {
	protocolAdapter
}

func init() {
	Register(chainsv1alpha2.ChainBerachain, &berachainAdapter{
		// livenessPort targets the EL reth JSON-RPC (must be co-deployed)
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

func (a *berachainAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainBerachain, client)
}

func (a *berachainAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "app.toml", berachainConfig, nil
}

func (a *berachainAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *berachainAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		// CometBFT P2P
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		// CometBFT RPC
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		// Engine API (connects to EL)
		{Name: "engine", ContainerPort: 8551, Protocol: corev1.ProtocolTCP},
		// Cosmos SDK telemetry (Prometheus)
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// ContainerCommand follows the upstream node setup: on first start it runs
// beacond init and fetches the mainnet genesis.json, config.toml and KZG
// trusted setup into /data/config. On every start the mounted app.toml
// replaces the generated one; beacond cannot start without a complete
// app.toml ("failed to unmarshal app config"). The Engine API JWT is taken
// from /jwt.hex when mounted (spec.extraVolumes), otherwise one is generated
// once at /data/config/jwt.hex for the EL to share.
func (a *berachainAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `set -e
NET=` + berachainNetworkFiles + `
if [ ! -f /data/config/genesis.json ] || [ ! -f /data/config/kzg-trusted-setup.json ]; then
  beacond init "${HOSTNAME:-chainplane}" --chain-id mainnet-beacon-80094 --beacon-kit.chain-spec mainnet --home /data >/dev/null 2>&1
  for f in genesis.json config.toml kzg-trusted-setup.json; do
    curl -fsSL -o "/data/config/$f" "$NET/$f"
  done
fi
cp /config/app.toml /data/config/app.toml
JWT=/jwt.hex
if [ ! -f "$JWT" ]; then
  JWT=/data/config/jwt.hex
  [ -f "$JWT" ] || beacond jwt generate -o "$JWT" --home /data
fi
exec beacond start --beacon-kit.engine.jwt-secret-path "$JWT" "$@"`
	return []string{"sh", "-c", script, "--"}
}

func (a *berachainAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--home", "/data", "--rpc.laddr", "tcp://0.0.0.0:26657"}
}

func (a *berachainAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *berachainAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "berachain/beacon-kit",
		TagPattern: `^v(?P<version>\d+\.\d+\.\d+)$`,
	}
}

const berachainConfig = `# BeaconKit consensus layer, Berachain mainnet (chain-spec "mainnet", 80094).
# Based on beacon-kit testing/networks/80094/app.toml. Copied over
# /data/config/app.toml on every start, so ConfigMap edits apply on restart.
# Requires a co-deployed EL (ghcr.io/berachain/bera-reth) at 127.0.0.1:8551.

pruning = "everything"
pruning-keep-recent = "0"
pruning-interval = "0"
halt-height = 0
halt-time = 0
min-retain-blocks = 0
inter-block-cache = true
index-events = []
iavl-cache-size = 781250
iavl-disable-fastnode = true
app-db-backend = "pebbledb"

[telemetry]
service-name = "beacond_node"
enabled = true
enable-hostname = true
enable-hostname-label = true
enable-service-label = true
prometheus-retention-time = 60
global-labels = []
metrics-sink = ""
statsd-addr = ""
datadog-hostname = ""

[beacon-kit]
chain-spec = "mainnet"
chain-spec-file = ""
shutdown-timeout = "5m0s"

[beacon-kit.engine]
rpc-dial-url = "http://127.0.0.1:8551"
rpc-timeout = "2s"
rpc-retry-interval = "100ms"
rpc-max-retry-interval = "10s"
rpc-startup-check-interval = "3s"
rpc-jwt-refresh-interval = "30s"
# Overridden at start: /jwt.hex when mounted, else a secret generated once
# at /data/config/jwt.hex.
jwt-secret-path = "/jwt.hex"

[beacon-kit.logger]
time-format = "RFC3339"
log-level = "info"
style = "json"

[beacon-kit.kzg]
trusted-setup-path = "/data/config/kzg-trusted-setup.json"
implementation = "crate-crypto/go-kzg-4844"

# Full node: block building is only needed on validators.
[beacon-kit.payload-builder]
enabled = false
suggested-fee-recipient = "0x0000000000000000000000000000000000000000"
payload-timeout = "850ms"

[beacon-kit.validator]
graffiti = ""
availability-window = "8192"

[beacon-kit.node-api]
enabled = true
address = "0.0.0.0:3500"
logging = false
`

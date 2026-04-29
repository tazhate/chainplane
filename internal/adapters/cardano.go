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
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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

type cardanoAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainCardano, &cardanoAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 12798},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string)
// --------------------------------------------------------------------------

const cardanoConfig = `{
  "Protocol": "Cardano",
  "RequiresNetworkMagic": "RequiresNoMagic",
  "ByronGenesisFile": "/config/byron-genesis.json",
  "ShelleyGenesisFile": "/config/shelley-genesis.json",
  "AlonzoGenesisFile": "/config/alonzo-genesis.json",
  "ConwayGenesisFile": "/config/conway-genesis.json",
  "SocketPath": "/data/node.socket",
  "DefaultBackends": ["KatipBK"],
  "DefaultScribes": [["StdoutSK", "stdout"]],
  "setupScribes": [{"scKind": "StdoutSK", "scName": "stdout", "scRotation": null}],
  "minSeverity": "Info",
  "TraceBlockFetchDecisions": false,
  "EnableP2P": true,
  "TargetNumberOfRootPeers": 100,
  "TargetNumberOfKnownPeers": 100,
  "TargetNumberOfEstablishedPeers": 50,
  "TargetNumberOfActivePeers": 20,
  "ExperimentalHardForksEnabled": false,
  "ExperimentalProtocolsEnabled": false
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *cardanoAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainCardano, client)
}

func (a *cardanoAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.json", cardanoConfig, nil
}

func (a *cardanoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// rpcURL is the service URL with the RPC port (e.g. :3001).
	// Cardano exposes Prometheus metrics on port 12798, so we must replace
	// the port rather than appending (which would produce an invalid URL like :3001:12798).
	metricsURL := rpcURL
	if !strings.Contains(metricsURL, "12798") {
		if u, err := url.Parse(rpcURL); err == nil {
			u.Host = u.Hostname() + ":12798"
			metricsURL = u.String()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL+"/metrics", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("scrape Cardano metrics endpoint: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return syncingPseudo(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("read Cardano metrics response: %w", err)
	}

	var blockNum, slotNum, replayProgress float64
	var cardanoPeers int32
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "cardano_node_metrics_blockNum_int ") {
			blockNum, _ = cardanoPromMetric(line)
		}
		if strings.HasPrefix(line, "cardano_node_metrics_slotNum_int ") {
			slotNum, _ = cardanoPromMetric(line)
		}
		if strings.HasPrefix(line, "cardano_node_metrics_blockReplayProgress_real ") {
			replayProgress, _ = cardanoPromMetric(line)
		}
		if strings.HasPrefix(line, "cardano_node_metrics_peerSelection_hot_peers_int ") {
			if v, ok := cardanoPromMetric(line); ok && v > 0 {
				cardanoPeers = int32(v)
			}
		}
	}

	if blockNum == 0 {
		p := pseudoBlock()
		return SyncStatus{IsSyncing: true, CurrentBlock: p, HighestBlock: p + 1, Progress: replayProgress}, nil
	}

	tipSlot := float64(cardanoNetworkTip())
	progress := slotNum / tipSlot * 100.0
	if progress > 100.0 {
		progress = 100.0
	}

	isSyncing := progress < 99.5
	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: int64(blockNum),
		HighestBlock: int64(blockNum / (progress / 100.0)),
		Progress:     progress,
		Peers:        cardanoPeers,
	}, nil
}

// StartupProbe gives Cardano up to 3h (360x30s) to complete the initial replay.
func (a *cardanoAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(12798, 60, 30, 10, 360)
}

// ContainerCommand downloads official IOHK mainnet configs to /data/configs/ (PVC).
func (a *cardanoAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `set -e
VERSION=10.6.2
BASE=https://raw.githubusercontent.com/IntersectMBO/cardano-node/${VERSION}/configuration/cardano
mkdir -p /data/configs
cd /data/configs
if [ "$(cat .version 2>/dev/null)" != "$VERSION" ]; then
  rm -f mainnet-config.json mainnet-topology.json mainnet-byron-genesis.json mainnet-shelley-genesis.json mainnet-alonzo-genesis.json mainnet-conway-genesis.json mainnet-checkpoints.json
fi
for f in mainnet-config.json mainnet-byron-genesis.json mainnet-shelley-genesis.json mainnet-alonzo-genesis.json mainnet-conway-genesis.json mainnet-checkpoints.json; do
  # -s checks non-empty to catch empty/partial files from interrupted downloads.
  if [ ! -s "$f" ]; then
    rm -f "$f"
    curl -fsSLk -o "$f" "$BASE/$f" || { rm -f "$f"; echo "[cardano] FAILED to download $f"; exit 1; }
    [ -s "$f" ] || { rm -f "$f"; echo "[cardano] $f empty after download — exiting for retry"; exit 1; }
  fi
done
cfg=$(cat mainnet-config.json)
cfg="${cfg/'"EnableP2P": false'/'"EnableP2P": true'}"
cfg="${cfg/127.0.0.1 12798/0.0.0.0 12798}"
printf '%s\n' "$cfg" > mainnet-config.json
printf '{"localRoots":[{"accessPoints":[{"address":"45.132.74.207","port":3001},{"address":"69.62.78.211","port":3001},{"address":"195.49.96.164","port":3001},{"address":"140.99.243.245","port":3001},{"address":"146.185.219.164","port":3001},{"address":"104.234.167.240","port":3001},{"address":"193.203.165.167","port":3001},{"address":"188.244.117.54","port":3001}],"advertise":false,"trustable":true,"valency":4}],"publicRoots":[],"useLedgerAfterSlot":-1}\n' > mainnet-topology.json
echo "$VERSION" > .version
exec cardano-node run \
  --config /data/configs/mainnet-config.json \
  --topology /data/configs/mainnet-topology.json \
  --database-path /data/db \
  --socket-path /data/node.socket \
  --port 3001`
	return []string{"bash", "-c", script}
}

func (a *cardanoAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "p2p", ContainerPort: 3001, Protocol: corev1.ProtocolTCP},
		{Name: "ekg", ContainerPort: 12788, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 12798, Protocol: corev1.ProtocolTCP},
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// cardanoNetworkTip returns the estimated current slot on Cardano mainnet.
// Cardano has exactly 1 slot/second since the Shelley era.
// Shelley genesis: slot 4,924,800 at 2020-07-29T21:44:51Z.
func cardanoNetworkTip() int64 {
	const shelleyStartSlot = int64(4_924_800)
	shelleyStart := time.Date(2020, 7, 29, 21, 44, 51, 0, time.UTC)
	elapsed := time.Since(shelleyStart).Seconds()
	if elapsed < 0 {
		return shelleyStartSlot
	}
	return shelleyStartSlot + int64(elapsed)
}

// cardanoPromMetric parses a single Prometheus metric line and returns its float64 value.
func cardanoPromMetric(line string) (float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (a *cardanoAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *cardanoAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "intersectmbo/cardano-node",
		TagPattern: `^\d+\.\d+`,
	}
}

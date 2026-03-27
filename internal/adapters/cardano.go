package adapters

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultCardanoImage = "ghcr.io/intersectmbo/cardano-node:10.6.2"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type cardanoAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainCardano, &cardanoAdapter{
		baseAdapter: baseAdapter{livenessPort: 12798},
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

func (a *cardanoAdapter) DefaultImage(_ string) string {
	return defaultCardanoImage
}

func (a *cardanoAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "config.json", cardanoConfig, nil
}

func (a *cardanoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// rpcURL points to http://localhost:12798 (Prometheus metrics port)
	metricsURL := rpcURL
	if !strings.Contains(metricsURL, "12798") {
		metricsURL = strings.TrimRight(rpcURL, "/") + ":12798"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL+"/metrics", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("cardano metrics request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return syncingPseudo(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("cardano metrics read: %w", err)
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

	// Cardano mainnet slot estimate. Epoch 616 started 2026-02-28;
	// actual tip ~181M as of 2026-03-15. Use 182M with headroom.
	const knownTipSlot float64 = 182_000_000
	progress := slotNum / knownTipSlot * 100.0
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
func (a *cardanoAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(12798, 60, 30, 10, 360)
}

// ContainerCommand downloads official IOHK mainnet configs to /data/configs/ (PVC).
func (a *cardanoAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	const script = `set -e
VERSION=10.6.2
BASE=https://raw.githubusercontent.com/IntersectMBO/cardano-node/${VERSION}/configuration/cardano
mkdir -p /data/configs
cd /data/configs
if [ "$(cat .version 2>/dev/null)" != "$VERSION" ]; then
  rm -f mainnet-config.json mainnet-topology.json mainnet-byron-genesis.json mainnet-shelley-genesis.json mainnet-alonzo-genesis.json mainnet-conway-genesis.json mainnet-checkpoints.json
fi
for f in mainnet-config.json mainnet-byron-genesis.json mainnet-shelley-genesis.json mainnet-alonzo-genesis.json mainnet-conway-genesis.json mainnet-checkpoints.json; do
  [ -f "$f" ] || curl -fsSLk -o "$f" "$BASE/$f"
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

func (a *cardanoAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "p2p", ContainerPort: 3001, Protocol: corev1.ProtocolTCP},
		{Name: "ekg", ContainerPort: 12788, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 12798, Protocol: corev1.ProtocolTCP},
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

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

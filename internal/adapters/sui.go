package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultSuiImage = "mysten/sui-node:v1.39.2"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type suiAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainSui, &suiAdapter{
		baseAdapter: baseAdapter{livenessPort: 9000},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string -- SUI fullnode.yaml is not parameterized)
// --------------------------------------------------------------------------

const suiConfig = `# Sui fullnode configuration
p2p-config:
  listen-address: "0.0.0.0:8084"
  seed-peers:
    # Official Mysten Labs seed peers (source: MystenLabs/sui docs, 2026-03-15)
    - peer-id: d32b55bdf1737ec415df8c88b3bf91e194b59ee3127e3f38ea46fd88ba2e7849
      address: "/dns/mel-00.mainnet.sui.io/udp/8084"
    - peer-id: c7bf6cb93ca8fdda655c47ebb85ace28e6931464564332bf63e27e90199c50ee
      address: "/dns/ewr-00.mainnet.sui.io/udp/8084"
    - peer-id: 3227f8a05f0faa1a197c075d31135a366a1c6f3d4872cb8af66c14dea3e0eb66
      address: "/dns/ewr-01.mainnet.sui.io/udp/8084"
    - peer-id: c619a5e0f8f36eac45118c1f8bda28f0f508e2839042781f1d4a9818043f732c
      address: "/dns/lhr-00.mainnet.sui.io/udp/8084"
    # Community SSF nodes
    - peer-id: 0c52ca8d2b9f51be4a50eb44ace863c05aadc940a7bd15d4d3f498deb81d7fc6
      address: "/dns/sui-mainnet-ssfn-1.nodeinfra.com/udp/8084"
    - peer-id: 1dbc28c105aa7eb9d1d3ac07ae663ea638d91f2b99c076a52bbded296bd3ed5c
      address: "/dns/sui-mainnet-ssfn-2.nodeinfra.com/udp/8084"
    - peer-id: 5ff8461ab527a8f241767b268c7aaf24d0312c7b923913dd3c11ee67ef181e45
      address: "/dns/sui-mainnet-ssfn-ashburn-na.overclock.run/udp/8084"
    - peer-id: e1a4f40d66f1c89559a195352ba9ff84aec28abab1d3aa1c491901a252acefa6
      address: "/dns/sui-mainnet-ssfn-dallas-na.overclock.run/udp/8084"
    - peer-id: fadb7ccb0b7fc99223419176e707f5122fef4ea686eb8e80d1778588bf5a0bcd
      address: "/dns/ssn01.mainnet.sui.rpcpool.com/udp/8084"
    - peer-id: 13783584a90025b87d4604f1991252221e5fd88cab40001642f4b00111ae9b7e
      address: "/dns/ssn02.mainnet.sui.rpcpool.com/udp/8084"

json-rpc-address: "0.0.0.0:9000"
metrics-address: "0.0.0.0:9184"
admin-interface-address: "0.0.0.0:1337"

db-path: /data/db
genesis:
  genesis-file-location: "/data/genesis.blob"

enable-experimental-rest-api: false

state-archive-read-config:
  - object-store-config:
      object-store: "GCS"
      bucket: "mysten-mainnet-checkpoints"
      no-sign-request: true
    concurrency: 10
    use-for-pruning-watermark: true
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *suiAdapter) DefaultImage(_ string) string {
	return defaultSuiImage
}

func (a *suiAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "fullnode.yaml", suiConfig, nil
}

// HealthCheck queries the Prometheus metrics endpoint (port 9184) for checkpoint progress.
func (a *suiAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	u, err := url.Parse(rpcURL)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("sui parse rpcURL: %w", err)
	}
	metricsURL := "http://" + u.Hostname() + ":9184/metrics"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("sui metrics request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return syncingPseudo(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("sui metrics read: %w", err)
	}

	var highestSynced, lastExecuted float64
	var suiPeers int32
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "highest_synced_checkpoint ") {
			if v, ok := parsePromMetric(line); ok {
				highestSynced = v
			}
		}
		if strings.HasPrefix(line, "last_executed_checkpoint ") {
			if v, ok := parsePromMetric(line); ok {
				lastExecuted = v
			}
		}
		if strings.HasPrefix(line, "sui_quinn_network_peers ") {
			if v, ok := parsePromMetric(line); ok {
				suiPeers = int32(v)
			}
		}
	}

	if highestSynced == 0 && lastExecuted == 0 {
		return syncingPseudo(), nil
	}

	current := int64(lastExecuted)
	if current == 0 {
		current = int64(highestSynced)
	}

	tip := suiNetworkTip(ctx)
	if tip == 0 {
		tip = current
	}
	var progress float64
	if tip > 0 {
		progress = float64(current) / float64(tip) * 100.0
		if progress > 100.0 {
			progress = 100.0
		}
	}
	isSyncing := current < tip
	highestBlock := tip
	if !isSyncing {
		highestBlock = current
	}

	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: current,
		HighestBlock: highestBlock,
		Progress:     progress,
		Peers:        suiPeers,
	}, nil
}

// StartupProbe gives SUI up to 1h (120x30s) to complete genesis loading.
func (a *suiAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(9000, 30, 30, 10, 120)
}

func (a *suiAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"sh", "-c",
		`[ -f /data/genesis.blob ] || curl -fsSL -o /data/genesis.blob https://raw.githubusercontent.com/MystenLabs/sui-genesis/main/mainnet/genesis.blob
exec /usr/local/bin/sui-node --config-path /config/fullnode.yaml`,
	}
}

// InitContainers adds a SUI formal snapshot download init container.
func (a *suiAdapter) InitContainers(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.Container {
	return []corev1.Container{
		{
			Name:  "sui-formal-snapshot",
			Image: "mysten/sui-tools:mainnet",
			Command: []string{"sh", "-c", `set -e
# Verify DB health: check that key RocksDB dirs have MANIFEST/CURRENT files.
# A missing CURRENT means the DB is corrupt (e.g. partial rm) — wipe and re-download.
if [ -d /data/db/live ]; then
  DB_OK=true
  for sub in checkpoints store/perpetual; do
    dir="/data/db/live/$sub"
    if [ -d "$dir" ] && [ ! -f "$dir/CURRENT" ]; then
      echo "WARN: $dir exists but CURRENT file missing — DB corrupt"
      DB_OK=false
    fi
  done
  if [ "$DB_OK" = true ] && [ -n "$(ls -A /data/db/live 2>/dev/null)" ]; then
    echo "SUI DB exists and looks healthy — skipping formal snapshot download."
    exit 0
  fi
  echo "Corrupt or incomplete DB detected — wiping /data/db for fresh snapshot..."
  rm -rf /data/db
fi

# Resume partial download: staging/ exists means a previous run was interrupted.
# sui-tool skips already-present SST files so re-running continues from where it left off.
if [ -d /data/db/staging ] || [ -d /data/db/snapshot ]; then
  echo "Partial snapshot detected ($(du -sh /data/db 2>/dev/null | cut -f1) downloaded) — resuming..."
  if sui-tool download-formal-snapshot \
    --path /data/db \
    --genesis /data/genesis.blob \
    --num-parallel-downloads 25 \
    --no-sign-request \
    --network mainnet \
    --latest; then
    echo "SUI formal snapshot resume complete."
    exit 0
  fi
  echo "Resume failed — wiping and starting fresh..."
  rm -rf /data/db
fi

echo "No existing DB — downloading latest SUI formal snapshot..."
sui-tool download-formal-snapshot \
  --path /data/db \
  --genesis /data/genesis.blob \
  --num-parallel-downloads 25 \
  --no-sign-request \
  --network mainnet \
  --latest || {
    echo "WARNING: Formal snapshot download failed (exit $?), node will sync from archive"
    exit 0
  }
echo "SUI formal snapshot restore complete."
`},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/data"},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
				},
			},
		},
	}
}

func (a *suiAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9184, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 8084, Protocol: corev1.ProtocolUDP},
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// suiNetworkTip queries the public Sui RPC for the latest checkpoint sequence number.
// Returns 0 on error — callers must treat 0 as "unknown tip" and fall back gracefully.
func suiNetworkTip(ctx context.Context) int64 {
	const suiPublicRPC = "https://sui-rpc.publicnode.com"
	body := `{"jsonrpc":"2.0","method":"suix_getLatestCheckpointSequenceNumber","params":[],"id":1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, suiPublicRPC, strings.NewReader(body))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := rpcClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0
	}
	var result struct{ Result string }
	if json.Unmarshal(data, &result) != nil {
		return 0
	}
	n, _ := strconv.ParseInt(result.Result, 10, 64)
	return n
}

// parsePromMetric parses a single Prometheus metric line and returns its float64 value.
func parsePromMetric(line string) (float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	var v float64
	if _, err := fmt.Sscanf(fields[1], "%f", &v); err != nil {
		return 0, false
	}
	return v, true
}

func (a *suiAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *suiAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "mysten/sui-node",
		TagPattern: `^mainnet-v\d+\.\d+\.\d+$`,
		TagPrefix:  "mainnet-",
	}
}

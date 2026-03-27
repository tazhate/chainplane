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
	"time"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultAptosImage = "aptoslabs/validator:mainnet"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type aptosAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainAptos, &aptosAdapter{
		baseAdapter: baseAdapter{livenessPort: 8080},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string)
// --------------------------------------------------------------------------

// genesis.blob and waypoint.txt are downloaded by ContainerCommand to /data/aptos-genesis/
const aptosConfig = `base:
  role: "full_node"
  data_dir: "/data"
  waypoint:
    from_file: "/data/aptos-genesis/waypoint.txt"

execution:
  genesis_file_location: "/data/aptos-genesis/genesis.blob"

full_node_networks:
  - network_id: "public"
    discovery_method: "onchain"
    listen_address: "/ip4/0.0.0.0/tcp/6180"
    seeds: {}
    max_inbound_connections: 100

api:
  enabled: true
  address: "0.0.0.0:8080"

state_sync:
  state_sync_driver:
    bootstrapping_mode: DownloadLatestStates
    continuous_syncing_mode: ExecuteTransactionsOrApplyOutputs

storage:
  rocksdb_configs:
    enable_storage_sharding: true
`

// --------------------------------------------------------------------------
// Response types
// --------------------------------------------------------------------------

type aptosLedgerResponse struct {
	LedgerVersion       string `json:"ledger_version"`
	OldestLedgerVersion string `json:"oldest_ledger_version"`
	NodeRole            string `json:"node_role"`
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *aptosAdapter) DefaultImage(_ string) string {
	return defaultAptosImage
}

func (a *aptosAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "fullnode.yaml", aptosConfig, nil
}

func (a *aptosAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/v1", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("aptos api request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("aptos api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("aptos api read: %w", err)
	}

	var ledger aptosLedgerResponse
	if err := json.Unmarshal(body, &ledger); err != nil {
		return SyncStatus{}, fmt.Errorf("aptos api parse: %w", err)
	}

	current, _ := strconv.ParseInt(ledger.LedgerVersion, 10, 64)

	// During state snapshot sync, ledger_version stays at 0 for hours.
	if current == 0 {
		return syncingPseudo(), nil
	}

	// best-effort peer count from Prometheus metrics (port 9101)
	peers := aptosPeerCount(ctx, rpcURL)

	return SyncStatus{
		IsSyncing:    false,
		CurrentBlock: current,
		HighestBlock: current,
		Progress:     100.0,
		Peers:        peers,
	}, nil
}

// StartupProbe gives Aptos up to 4h (480x30s) to complete snapshot bootstrap.
func (a *aptosAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return httpProbe("/v1/-/healthy", 8080, 60, 30, 10, 480)
}

func (a *aptosAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return httpProbe("/v1/-/healthy", 8080, 60, 30, 10, 5)
}

func (a *aptosAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	const script = `BASE=https://raw.githubusercontent.com/aptos-labs/aptos-networks/main/mainnet
mkdir -p /data/aptos-genesis
[ -f /data/aptos-genesis/genesis.blob ] || curl -fsSL -o /data/aptos-genesis/genesis.blob "$BASE/genesis.blob"
[ -f /data/aptos-genesis/waypoint.txt ] || curl -fsSL -o /data/aptos-genesis/waypoint.txt "$BASE/waypoint.txt"
exec aptos-node --config /config/fullnode.yaml`
	return []string{"sh", "-c", script}
}

func (a *aptosAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9101, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 6180, Protocol: corev1.ProtocolTCP},
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// aptosPeerCount reads inbound connection count from Prometheus metrics.
func aptosPeerCount(ctx context.Context, rpcURL string) int32 {
	u, err := url.Parse(rpcURL)
	if err != nil {
		return 0
	}
	metricsURL := "http://" + u.Hostname() + ":9101/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return 0
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var peers int32
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `aptos_connections{`) && strings.Contains(line, `direction="inbound"`) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseFloat(fields[len(fields)-1], 64); err == nil {
					peers += int32(v)
				}
			}
		}
	}
	return peers
}

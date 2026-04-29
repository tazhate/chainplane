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
	"encoding/json"
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

type aptosAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainAptos, &aptosAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8080},
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

func (a *aptosAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainAptos, client)
}

func (a *aptosAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "fullnode.yaml", aptosConfig, nil
}

func (a *aptosAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/v1", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("call Aptos REST API: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("aptos api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("read Aptos API response: %w", err)
	}

	var ledger aptosLedgerResponse
	if err := json.Unmarshal(body, &ledger); err != nil {
		return SyncStatus{}, fmt.Errorf("decode Aptos API payload: %w", err)
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
func (a *aptosAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return httpProbe("/v1/-/healthy", 8080, 60, 30, 10, 480)
}

func (a *aptosAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return httpProbe("/v1/-/healthy", 8080, 60, 30, 10, 5)
}

func (a *aptosAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `BASE=https://raw.githubusercontent.com/aptos-labs/aptos-networks/main/mainnet
mkdir -p /data/aptos-genesis
[ -f /data/aptos-genesis/genesis.blob ] || curl -fsSL -o /data/aptos-genesis/genesis.blob "$BASE/genesis.blob"
[ -f /data/aptos-genesis/waypoint.txt ] || curl -fsSL -o /data/aptos-genesis/waypoint.txt "$BASE/waypoint.txt"
exec aptos-node --config /config/fullnode.yaml`
	return []string{"sh", "-c", script}
}

func (a *aptosAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9101, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 6180, Protocol: corev1.ProtocolTCP},
	}
}

func (a *aptosAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
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

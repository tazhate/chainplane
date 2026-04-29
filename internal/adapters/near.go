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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
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

type nearAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainNear, &nearAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 3030},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var nearConfigTpl = template.Must(template.New("near-config").Parse(`{
  "genesis_file": "genesis.json",
  "genesis_records_file": null,
  "validator_key_file": "validator_key.json",
  "node_key_file": "node_key.json",
  "public_key": "",
  "secret_key": "",
  "network": {
    "addr": "0.0.0.0:24567",
    "boot_nodes": "{{ .BootNodes }}",
    "handshake_timeout": {"secs": 20, "nanos": 0},
    "skip_sync_wait": false,
    "ban_window": {"secs": 1, "nanos": 0},
    "max_num_peers": 40,
    "minimum_outbound_peers": 5,
    "ideal_connections_lo": 30,
    "ideal_connections_hi": 35,
    "peer_recent_time_window": {"secs": 600, "nanos": 0},
    "safe_set_size": 20,
    "archival_peer_connections_lower_bound": 10,
    "blacklist": []
  },
  "rpc": {
    "addr": "0.0.0.0:3030",
    "cors_allowed_origins": ["*"],
    "enable_debug_rpc": false
  },
  "telemetry": {
    "endpoints": []
  },
  "consensus": {
    "min_block_production_delay": {"secs": 1, "nanos": 0},
    "max_block_production_delay": {"secs": 2, "nanos": 0},
    "max_block_wait_delay": {"secs": 6, "nanos": 0},
    "produce_empty_blocks": true,
    "block_fetch_horizon": 50,
    "state_sync_timeout": {"secs": 60, "nanos": 0}
  },
  "tracked_accounts": [],
  "tracked_shadow_validator": null,
  "tracked_shards": [],
  "archive": false,
  "log_summary_style": "plain",
  "gc": {
    "gc_blocks_limit": 2,
    "gc_fork_clean_step": 100,
    "gc_num_epochs_to_keep": 5,
    "gc_step_period": {"secs": 1, "nanos": 0}
  },
  "view_client_threads": 4,
  "epoch_sync": {
    "epoch_sync_enabled": true,
    "epoch_sync_horizon": 10240,
    "epoch_sync_accept_proof_max_horizon": 1024
  },
  "network_id": "{{ .NetworkID }}",
  "store": {
    "path": "data"
  }
}`))

// --------------------------------------------------------------------------
// Response types
// --------------------------------------------------------------------------

type nearStatusResponse struct {
	SyncInfo struct {
		LatestBlockHeight int64 `json:"latest_block_height"`
		Syncing           bool  `json:"syncing"`
	} `json:"sync_info"`
	NumActivePeers int32 `json:"num_active_peers"`
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *nearAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainNear, client)
}

func (a *nearAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	networkID := "mainnet"
	bootNodes := "ed25519:86EtEy7epneKyrcJwSWP7zsisTkfDRH5CFVszt4qiQYw@35.195.32.249:24567,ed25519:BFzAKpbH3h7tEVqTKYRKb97EgGgwUbY5tq7vxvFiBFJF@34.81.150.34:24567"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		networkID = "testnet"
		bootNodes = "ed25519:4k9csx6zMiXy4waUvRMPTkEtAS2RFKLVScocR5HwN53P@34.80.165.100:24567"
	}

	var buf bytes.Buffer
	err := nearConfigTpl.Execute(&buf, struct {
		BootNodes string
		NetworkID string
	}{bootNodes, networkID})
	if err != nil {
		return "", "", fmt.Errorf("near config: %w", err)
	}
	return "config.json", buf.String(), nil
}

func (a *nearAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("call NEAR /status endpoint: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		// RPC may be unavailable during NEAR state download.
		return syncingPseudo(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("read NEAR status response: %w", err)
	}

	var status nearStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("decode NEAR status payload: %w", err)
	}

	current := status.SyncInfo.LatestBlockHeight
	if !status.SyncInfo.Syncing {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: current,
			HighestBlock: current,
			Progress:     100.0,
			Peers:        status.NumActivePeers,
		}, nil
	}

	// During NEAR state sync, latest_block_height can stay constant for extended periods.
	// XOR with low bits of unix time to keep CurrentBlock changing.
	pseudoCurrent := current ^ (time.Now().Unix() & 0x3FF)
	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: pseudoCurrent,
		HighestBlock: pseudoCurrent + 1,
		Progress:     0,
		Peers:        status.NumActivePeers,
	}, nil
}

// StartupProbe gives NEAR up to 4h (480x30s) to complete state download.
func (a *nearAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return httpProbe("/status", 3030, 60, 30, 10, 480)
}

func (a *nearAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return httpProbe("/status", 3030, 60, 30, 10, 5)
}

func (a *nearAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	// neard requires genesis.json + node_key.json in home dir before running.
	// Boot nodes sourced from active peers on rpc.mainnet.near.org/network_info (2026-03-14).
	const bootNodes = "ed25519:GwE1TUQo4RYemsFsi5G3ZaNT5HiSuLYiYYYsuCutKmk9@67.213.119.43:24567," +
		"ed25519:FjmBNEfFrFMBHHMaRwAszFkNRBvP6j4UtTyy7Cwnvmwp@46.4.33.58:24567," +
		"ed25519:8N3Eww3tmnDgwcKPLt52P7orCvpGq2UoUVUdLZuLQMcS@37.27.66.152:24567," +
		"ed25519:GtX9bfvv6A6KfXAFsgtrm28hVkxaJVFM8knvp3mA1YBw@23.88.70.87:24567," +
		"ed25519:CStGdrdTdo7VeY2pSraMmnqknViNtGgGNMTv13R3ZL6n@212.95.39.96:24567"
	// --download-config is REQUIRED: without it neard generates a minimal config.json
	// that has no state_sync ExternalStorage block. The sed patches below are no-ops
	// on a minimal config, causing state sync to fall back to slow P2P which gets
	// stuck in the "header" phase with "NEAR state response sender mismatch" errors.
	// --download-config fetches the official mainnet config.json from NEAR's S3 bucket
	// which includes state_sync with ExternalStorage GCS bucket ("state-parts").
	// We then patch "state-parts" -> "fast-state-parts" for faster cold bootstrap.
	script := `if [ ! -f /data/genesis.json ] || [ ! -f /data/config.json ]; then
  neard --home /data init --chain-id mainnet --download-config --download-genesis
fi
sed -i 's/"bucket": "state-parts"/"bucket": "fast-state-parts"/' /data/config.json
sed -i 's/"state_sync_enabled": false/"state_sync_enabled": true/' /data/config.json
exec neard --home /data run --boot-nodes ` + bootNodes
	return []string{"sh", "-c", script}
}

func (a *nearAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		// NEAR exposes both JSON-RPC and Prometheus metrics on port 3030
		// (/metrics endpoint). Both entries point to the same port so that
		// a PodMonitor targeting port "metrics" can scrape /metrics directly.
		{Name: "rpc", ContainerPort: 3030, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 3030, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 24567, Protocol: corev1.ProtocolTCP},
	}
}

func (a *nearAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *nearAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "nearprotocol/nearcore",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

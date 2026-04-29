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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

type avalancheAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainAvalanche, &avalancheAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 9650},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var avaxConfigTpl = template.Must(template.New("avax-config").Parse(`{
  "network-id": "{{ .Network }}",
  "http-host": "0.0.0.0",
  "http-port": 9650,
  "http-allowed-hosts": "*",
  "data-dir": "/data",
  "api-admin-enabled": false,
  "index-enabled": false
}`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *avalancheAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainAvalanche, client)
}

func (a *avalancheAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	network := "mainnet"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		network = "fuji"
	}
	var buf bytes.Buffer
	if err := avaxConfigTpl.Execute(&buf, struct{ Network string }{network}); err != nil {
		return "", "", fmt.Errorf("avalanche config: %w", err)
	}
	return "config.json", buf.String(), nil
}

// ContainerCommand passes --config-file so avalanchego actually reads the operator config.
func (a *avalancheAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"/avalanchego/build/avalanchego", "--config-file=/config/config.json"}
}

// avaxHealthResponse is the relevant subset of GET /ext/health response.
type avaxHealthResponse struct {
	Healthy bool `json:"healthy"`
	Checks  map[string]struct {
		Error   string `json:"error,omitempty"`
		Message any    `json:"message,omitempty"`
	} `json:"checks"`
}

// HealthCheck first probes /ext/health to detect bootstrap phase.
// While bootstrapping it returns Syncing with estimated progress so the
// operator does not mark the node Degraded during the multi-hour initial sync.
// Once all subnets are bootstrapped it delegates to the C-Chain EVM RPC.
func (a *avalancheAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/ext/health", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("call Avalanche /health endpoint: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		// /ext/health accepts TCP but may not respond during heavy init (chain loading).
		if isContextTimeout(err) {
			return SyncStatus{IsSyncing: true, StallExempt: true, Progress: 1.0}, nil
		}
		return SyncStatus{}, fmt.Errorf("avax health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var health avaxHealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return SyncStatus{}, fmt.Errorf("decode Avalanche health payload: %w", err)
	}

	// If bootstrapped check is still failing, node is in bootstrap phase.
	if bc, ok := health.Checks["bootstrapped"]; ok && bc.Error != "" {
		fetched, finished := avaxBootstrapMetrics(ctx, rpcURL)
		// During DB compaction, bs_fetched freezes -- use time as a heartbeat.
		current := fetched
		if !finished && fetched > 0 {
			current = fetched ^ (time.Now().Unix() & 0xFFFF)
		}
		return SyncStatus{
			IsSyncing:    true,
			CurrentBlock: current,
			HighestBlock: current + 1,
			Progress:     0,
			Peers:        avaxPeerCount(ctx, rpcURL),
		}, nil
	}

	// Fully bootstrapped -- check C-Chain EVM RPC.
	status, err := evmHealthCheck(ctx, rpcURL+"/ext/bc/C/rpc")
	if err != nil {
		return status, err
	}
	status.Peers = avaxPeerCount(ctx, rpcURL)
	return status, nil
}

// StartupProbe gives Avalanche up to 2h (240x30s) to complete bootstrapping.
func (a *avalancheAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(9650, 30, 30, 10, 240)
}

func (a *avalancheAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9650, Protocol: corev1.ProtocolTCP},
		// metrics are served at /ext/metrics on the same HTTP port as RPC
		{Name: "metrics", ContainerPort: 9650, Protocol: corev1.ProtocolTCP},
		{Name: "staking", ContainerPort: 9651, Protocol: corev1.ProtocolTCP},
	}
}

func (a *avalancheAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *avalancheAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "avaplatform/avalanchego",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// avaxPeerCount calls info.peers to get the number of connected peers.
func avaxPeerCount(ctx context.Context, rpcURL string) int32 {
	body, _ := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", Method: "info.peers", ID: 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL+"/ext/info", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		Result struct {
			Peers []json.RawMessage `json:"peers"`
		} `json:"result"`
	}
	if b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes)); json.Unmarshal(b, &result) == nil {
		return int32(len(result.Result.Peers))
	}
	return 0
}

// avaxBootstrapMetrics reads the Prometheus metrics endpoint and returns
// (bsFetched, bootstrapFinished) for the P-Chain.
func avaxBootstrapMetrics(ctx context.Context, rpcURL string) (fetched int64, finished bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/ext/metrics", nil)
	if err != nil {
		return 0, false
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.HasPrefix(line, `avalanche_snowman_bs_fetched{chain="P"}`) {
			if f, err := strconv.ParseFloat(fields[1], 64); err == nil {
				fetched = int64(f)
			}
		}
		if strings.HasPrefix(line, `avalanche_snowman_bootstrap_finished{chain="P"}`) {
			if f, err := strconv.ParseFloat(fields[1], 64); err == nil {
				finished = f == 1
			}
		}
	}
	return fetched, finished
}

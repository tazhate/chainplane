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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

type ethereumBeaconAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainEthereumBeacon, &ethereumBeaconAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 5052},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *ethereumBeaconAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainEthereumBeacon, client)
}

func (a *ethereumBeaconAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "lighthouse.toml", ethereumBeaconConfig, nil
}

func (a *ethereumBeaconAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return beaconHealthCheck(ctx, rpcURL)
}

func (a *ethereumBeaconAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(5052, 300, 30, 10, 5)
}

func (a *ethereumBeaconAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return beaconPorts()
}

func (a *ethereumBeaconAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *ethereumBeaconAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "sigp/lighthouse",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Beacon health check (shared with gnosis-beacon)
// --------------------------------------------------------------------------

// beaconSyncingResponse maps the Beacon REST API /eth/v1/node/syncing response.
type beaconSyncingResponse struct {
	Data struct {
		IsSyncing    bool   `json:"is_syncing"`
		IsOptimistic bool   `json:"is_optimistic"`
		HeadSlot     string `json:"head_slot"`
		SyncDistance string `json:"sync_distance"`
	} `json:"data"`
}

// beaconHealthCheck calls the Beacon REST API /eth/v1/node/syncing endpoint.
// It is shared by both ethereum-beacon and gnosis-beacon adapters.
func beaconHealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	url := rpcURL + "/eth/v1/node/syncing"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("beacon health check: create request: %w", err)
	}

	resp, err := rpcClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return SyncStatus{IsSyncing: true, StallExempt: true}, nil
		}
		return SyncStatus{}, fmt.Errorf("beacon health check: %w: %w", ErrRPCUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("beacon health check: read response: %w", err)
	}

	var syncResp beaconSyncingResponse
	if err := json.Unmarshal(body, &syncResp); err != nil {
		return SyncStatus{}, fmt.Errorf("beacon health check: parse response: %w", err)
	}

	headSlot, err := strconv.ParseInt(syncResp.Data.HeadSlot, 10, 64)
	if err != nil {
		headSlot = 0
	}

	syncDistance, err := strconv.ParseInt(syncResp.Data.SyncDistance, 10, 64)
	if err != nil {
		syncDistance = 0
	}

	highestBlock := headSlot + syncDistance
	currentBlock := headSlot
	progress := progressFromBlocks(currentBlock, highestBlock)

	return SyncStatus{
		IsSyncing:    syncResp.Data.IsSyncing,
		CurrentBlock: currentBlock,
		HighestBlock: highestBlock,
		Progress:     progress,
	}, nil
}

// beaconPorts returns the standard Lighthouse beacon node container ports.
func beaconPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "api", ContainerPort: 5052, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 5054, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 9000, Protocol: corev1.ProtocolUDP},
	}
}

// --------------------------------------------------------------------------
// Config (Lighthouse TOML for Ethereum mainnet)
// --------------------------------------------------------------------------

const ethereumBeaconConfig = `# Lighthouse beacon node config for Ethereum
network = "mainnet"
datadir = "/data"
http = true
http-address = "0.0.0.0"
http-port = 5052
metrics = true
metrics-address = "0.0.0.0"
metrics-port = 5054
port = 9000
discovery-port = 9000
`

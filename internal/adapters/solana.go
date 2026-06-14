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
	"text/template"

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

type solanaAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainSolana, &solanaAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8899},
	})
}

// --------------------------------------------------------------------------
// Config template (parsed once)
// --------------------------------------------------------------------------

var solConfigTpl = template.Must(template.New("solana").Parse(`ledger-path: /data/ledger
rpc-port: 8899
rpc-bind-address: 0.0.0.0
full-rpc-api: true
no-voting: true
known-validator:
  - "{{ .Entrypoint }}"
entrypoint: "{{ .Entrypoint }}"
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *solanaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainSolana, client)
}

func (a *solanaAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	entrypoint := "entrypoint.mainnet-beta.solana.com:8001"
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		entrypoint = "entrypoint.testnet.solana.com:8001"
	}
	var buf bytes.Buffer
	if err := solConfigTpl.Execute(&buf, struct{ Entrypoint string }{entrypoint}); err != nil {
		return "", "", fmt.Errorf("solana config: %w", err)
	}
	return "validator.yml", buf.String(), nil
}

func (a *solanaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	result, err := callRPC(ctx, rpcURL, "getSlot", []any{map[string]string{"commitment": "finalized"}})
	if err != nil {
		return SyncStatus{}, fmt.Errorf("getSlot: %w", err)
	}
	var slot int64
	if err := json.Unmarshal(result, &slot); err != nil {
		return SyncStatus{}, fmt.Errorf("parse getSlot: %w", err)
	}

	// Get epoch info for progress (best-effort)
	epochResult, err := callRPC(ctx, rpcURL, "getEpochInfo", nil)
	if err != nil {
		return SyncStatus{IsSyncing: false, CurrentBlock: slot}, nil
	}
	var epochInfo struct {
		SlotIndex        int64 `json:"slotIndex"`
		SlotsInEpoch     int64 `json:"slotsInEpoch"`
		AbsoluteSlotDiff int64 `json:"absoluteSlot"`
	}
	_ = json.Unmarshal(epochResult, &epochInfo)

	return SyncStatus{
		IsSyncing:    false,
		CurrentBlock: slot,
		HighestBlock: slot,
		Progress:     100.0,
	}, nil
}

// StartupProbe gives Solana up to 1h (120x30s) to complete snapshot download
// before liveness probing begins.
func (a *solanaAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(8899, 30, 30, 10, 120)
}

func (a *solanaAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8899, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8900, Protocol: corev1.ProtocolTCP},
		{Name: "gossip", ContainerPort: 8001, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
	}
}

func (a *solanaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("16"),
		MemoryRequest: resource.MustParse("64Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *solanaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "anzaxyz/agave",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// Sidecars returns a solana-exporter sidecar that connects to the local RPC
// and exposes Prometheus metrics on port 8080.
func (a *solanaAdapter) Sidecars(_ chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return []corev1.Container{
		{
			Name:  "metrics-exporter",
			Image: "nordstroem/solana-exporter:latest",
			Args:  []string{"-rpcURI", "http://localhost:8899", "-addr", ":8080"},
			Ports: []corev1.ContainerPort{
				{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

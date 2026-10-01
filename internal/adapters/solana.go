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

// solanaCluster holds the public values an RPC node needs to join a cluster
// (docs.anza.xyz/clusters/available).
type solanaCluster struct {
	Entrypoints     []string
	KnownValidators []string
	GenesisHash     string
}

var (
	solanaMainnet = solanaCluster{
		Entrypoints: []string{
			"entrypoint.mainnet-beta.solana.com:8001",
			"entrypoint2.mainnet-beta.solana.com:8001",
			"entrypoint3.mainnet-beta.solana.com:8001",
			"entrypoint4.mainnet-beta.solana.com:8001",
			"entrypoint5.mainnet-beta.solana.com:8001",
		},
		KnownValidators: []string{
			"7Np41oeYqPefeNQEHSv1UDhYrehxin3NStELsSKCT4K2",
			"GdnSyH3YtwcxFvQrVVJMm1JhTS4QVX7MFsX56uJLUfiZ",
			"DE1bawNcRJB9rVm3buyMVfr8mBEoyyu73NBovf2oXJsJ",
			"CakcnaRDHka2gXyfbEd2d3xsvkJkqsLw2akB3zsN1D2S",
		},
		GenesisHash: "5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d",
	}
	solanaTestnet = solanaCluster{
		Entrypoints: []string{
			"entrypoint.testnet.solana.com:8001",
			"entrypoint2.testnet.solana.com:8001",
			"entrypoint3.testnet.solana.com:8001",
		},
		KnownValidators: []string{
			"5D1fNXzvv5NjV1ysLjirC4WY92RNsVH18vjmcszZd8on",
			"dDzy5SR3AXdYWVqbDEkVFdvSPCtS9ihF5kJkHCtXoFs",
			"Ft5fbkqNa76vnsjYNwjDZUXoTWpP7VYm3mtsaQckQADN",
			"eoKpUABi59aT4rR9HGS3LcMecfut9x7zJyodWWP43YQ",
			"9QxCLckBiJc783jnMvXZubK4wH86Eqqvashtrwvcsgkv",
		},
		GenesisHash: "4uhcVJyU9pJkvQyS88uRDiswHXSCkY3zQawwpjk2NsNY",
	}
)

// solanaExporterImage is the maintained solana-exporter (Asymmetric Research,
// successor of certusone/solana_exporter).
const solanaExporterImage = "ghcr.io/asymmetric-research/solana-exporter:v3.1.0"

func solanaClusterFor(spec chainsv1alpha2.ChainInstanceSpec) solanaCluster {
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		return solanaTestnet
	}
	return solanaMainnet
}

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

// solConfigTpl documents the cluster ContainerArgs joins. agave-validator
// takes flags only, so the mounted file is not read.
var solConfigTpl = template.Must(template.New("solana").Parse(`# agave-validator RPC node. Reference only: the validator reads CLI flags,
# set by the operator from the same values.
ledger-path: /data/ledger
identity: /data/identity.json
rpc-port: 8899
rpc-bind-address: 0.0.0.0
full-rpc-api: true
no-voting: true
expected-genesis-hash: "{{ .GenesisHash }}"
known-validator:
{{- range .KnownValidators }}
  - "{{ . }}"
{{- end }}
entrypoint:
{{- range .Entrypoints }}
  - "{{ . }}"
{{- end }}
`))

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *solanaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainSolana, client)
}

func (a *solanaAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	var buf bytes.Buffer
	if err := solConfigTpl.Execute(&buf, solanaClusterFor(spec)); err != nil {
		return "", "", fmt.Errorf("solana config: %w", err)
	}
	return "validator.yml", buf.String(), nil
}

// ContainerCommand replaces the image entrypoint, solana-run.sh, which builds
// a throwaway single-node dev cluster (own genesis, faucet, bootstrap
// validator) instead of joining a public one. The node identity keypair is
// generated once on the data volume, then agave-validator runs with
// ContainerArgs.
func (a *solanaAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	const script = `set -e
[ -f /data/identity.json ] || solana-keygen new --no-passphrase --silent -o /data/identity.json
exec agave-validator --identity /data/identity.json "$@"`
	return []string{"sh", "-c", script, "--"}
}

// ContainerArgs runs a non-voting RPC node. --private-rpc keeps the pod-local
// RPC address out of gossip; --log - sends logs to stdout instead of a file
// in the working directory; --no-os-network-limits-test skips the sysctl
// check a pod cannot satisfy without privileges; --no-port-check skips the
// entrypoint's reachability probe of the UDP ports, which fails behind the
// pod network's NAT and holds startup for minutes.
func (a *solanaAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	c := solanaClusterFor(spec)
	args := make([]string, 0, 24+2*len(c.Entrypoints)+2*len(c.KnownValidators))
	args = append(args,
		"--ledger", "/data/ledger",
		"--no-voting",
		"--rpc-port", "8899",
		"--rpc-bind-address", "0.0.0.0",
		"--full-rpc-api",
		"--private-rpc",
		"--gossip-port", "8001",
		"--dynamic-port-range", "8002-8032",
		"--expected-genesis-hash", c.GenesisHash,
		"--only-known-rpc",
		"--wal-recovery-mode", "skip_any_corrupted_record",
		"--limit-ledger-size",
		"--no-os-network-limits-test",
		"--no-port-check",
		"--log", "-",
	)
	for _, e := range c.Entrypoints {
		args = append(args, "--entrypoint", e)
	}
	for _, v := range c.KnownValidators {
		args = append(args, "--known-validator", v)
	}
	return args
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
// and exposes Prometheus metrics on port 8080. Light mode reports only the
// queried node, which is all a non-voting RPC node has.
func (a *solanaAdapter) Sidecars(_ chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return []corev1.Container{
		{
			Name:  "metrics-exporter",
			Image: solanaExporterImage,
			Args:  []string{"-rpc-url", "http://localhost:8899", "-listen-address", ":8080", "-light-mode"},
			Ports: []corev1.ContainerPort{
				{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

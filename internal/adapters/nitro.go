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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// --------------------------------------------------------------------------
// Shared Arbitrum Nitro (Arbitrum One and Orbit chains) node setup
// --------------------------------------------------------------------------

const (
	// nitroConfigFile is the Nitro --conf.file rendered into the config ConfigMap.
	nitroConfigFile = "nitro.json"
	// nitroMetricsPort is Nitro's default --metrics-server.port.
	nitroMetricsPort = int32(6070)
	// defaultNitroBeaconURL is the Ethereum beacon API Nitro reads blob
	// batches from when the parent chain is Ethereum (ethereum-beacon adapter).
	defaultNitroBeaconURL = "http://ethereum-beacon:5052"
)

// nitroChain describes how one Nitro chain is run. Everything except the
// parent chain endpoints goes into the --conf.file JSON; the endpoints come
// from env vars so they can be overridden with spec.extraEnv.
type nitroChain struct {
	// ChainID selects a chain built into Nitro (arb1, nova) or, with
	// ChainInfoJSON, an Orbit chain.
	ChainID uint64
	// ChainInfoJSON is the Orbit chain info (--chain.info-json); empty for
	// chains Nitro knows.
	ChainInfoJSON string
	// ForwardingTarget is the sequencer RPC that eth_sendRawTransaction is
	// forwarded to; empty for built-in chains, which carry it in their info.
	ForwardingTarget string
	// FeedURL is the sequencer feed; empty for built-in chains.
	FeedURL string
	// DASRestURL enables AnyTrust and reads batch data from this REST
	// aggregator.
	DASRestURL string
	// ParentChainURL is the default of $(L1_RPC_URL).
	ParentChainURL string
	// BlobsFromBeacon is set when the parent chain is Ethereum: batches posted
	// as blobs are read from $(L1_BEACON_URL).
	BlobsFromBeacon bool
	// HTTPPort and WSPort are the JSON-RPC ports exposed by the adapter.
	HTTPPort, WSPort int32
}

// nitroConfig renders the --conf.file JSON. Nitro rejects unknown keys, so
// only flags that exist in nitro-node are set. Chain state lives under the
// data volume: persistent.global-config=/data puts the chain directory at
// /data/<chain name>, not in the image's ~/.arbitrum.
func nitroConfig(c nitroChain) (string, error) {
	chain := map[string]any{"id": c.ChainID}
	if c.ChainInfoJSON != "" {
		chain["info-json"] = c.ChainInfoJSON
	}
	cfg := map[string]any{
		"chain": chain,
		"http": map[string]any{
			"addr":       "0.0.0.0",
			"port":       c.HTTPPort,
			"vhosts":     []string{"*"},
			"corsdomain": []string{"*"},
			"api":        []string{"eth", "net", "web3", "arb", "debug"},
		},
		"ws": map[string]any{
			"addr":    "0.0.0.0",
			"port":    c.WSPort,
			"origins": []string{"*"},
			"api":     []string{"eth", "net", "web3", "arb"},
		},
		"persistent": map[string]any{"global-config": "/data"},
		"metrics":    true,
		"metrics-server": map[string]any{
			"addr": "0.0.0.0",
			"port": nitroMetricsPort,
		},
	}
	if c.ForwardingTarget != "" {
		cfg["execution"] = map[string]any{"forwarding-target": c.ForwardingTarget}
	}
	// The operator runs RPC nodes: staker.enable defaults to true (watchtower
	// validator), which upstream Orbit node guides switch off.
	node := map[string]any{"staker": map[string]any{"enable": false}}
	if c.FeedURL != "" {
		node["feed"] = map[string]any{"input": map[string]any{"url": []string{c.FeedURL}}}
	}
	if c.DASRestURL != "" {
		// node.da.anytrust replaces the deprecated node.data-availability
		// (nitro v3.10+).
		node["da"] = map[string]any{"anytrust": map[string]any{
			"enable":          true,
			"rest-aggregator": map[string]any{"enable": true, "urls": []string{c.DASRestURL}},
		}}
	}
	cfg["node"] = node
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("nitro config: %w", err)
	}
	return string(out) + "\n", nil
}

// nitroArgs points Nitro at the rendered config and the parent chain. Nitro
// has no geth-style --metrics.addr/--metrics.port: metrics are configured
// in the config file (metrics-server.*).
func nitroArgs(c nitroChain) []string {
	args := []string{
		"--conf.file=/config/" + nitroConfigFile,
		"--parent-chain.connection.url=$(L1_RPC_URL)",
	}
	if c.BlobsFromBeacon {
		args = append(args, "--parent-chain.blob-client.beacon-url=$(L1_BEACON_URL)")
	}
	return args
}

// nitroEnv sets the parent chain endpoints used by nitroArgs.
func nitroEnv(c nitroChain) []corev1.EnvVar {
	env := []corev1.EnvVar{{Name: "L1_RPC_URL", Value: c.ParentChainURL}}
	if c.BlobsFromBeacon {
		env = append(env, corev1.EnvVar{Name: "L1_BEACON_URL", Value: defaultNitroBeaconURL})
	}
	return env
}

// nitroPorts returns the Nitro container ports: JSON-RPC, WS and metrics.
// Nitro has no devp2p, so there is no P2P port.
func nitroPorts(c nitroChain) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: c.HTTPPort, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: c.WSPort, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: nitroMetricsPort, Protocol: corev1.ProtocolTCP},
	}
}

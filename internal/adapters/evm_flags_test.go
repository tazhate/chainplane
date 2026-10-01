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
package adapters_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// evmFlagsCommandLine returns the adapter's command followed by its args, the
// way the kubelet joins them.
func evmFlagsCommandLine(t *testing.T, chain chainsv1alpha2.Chain, network chainsv1alpha2.Network) []string {
	t.Helper()
	adapter, ok := adapters.Get(chain)
	if !ok {
		t.Fatalf("%s adapter not registered", chain)
	}
	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chain, Network: network}
	var line []string
	if p, ok := adapter.(adapters.ContainerCommandProvider); ok {
		line = append(line, p.ContainerCommand(spec)...)
	}
	if p, ok := adapter.(adapters.ContainerArgsProvider); ok {
		line = append(line, p.ContainerArgs(spec)...)
	}
	return line
}

// Nethermind, Besu, Nitro, Bor and l2geth (geth 1.9) reject geth's
// --metrics.addr/--metrics.port and exit or print their usage.
func TestNonGethClientsHaveNoGethMetricsFlags(t *testing.T) {
	for _, chain := range []chainsv1alpha2.Chain{
		chainsv1alpha2.ChainGnosis,
		chainsv1alpha2.ChainFuse,
		chainsv1alpha2.ChainPolygon,
		chainsv1alpha2.ChainMetis,
		chainsv1alpha2.ChainArbitrum,
		chainsv1alpha2.ChainEverclear,
		chainsv1alpha2.ChainPlaynance,
		chainsv1alpha2.ChainGravityAlpha,
		chainsv1alpha2.ChainPlume,
	} {
		t.Run(string(chain), func(t *testing.T) {
			for _, arg := range evmFlagsCommandLine(t, chain, chainsv1alpha2.NetworkMainnet) {
				name, _, _ := strings.Cut(arg, "=")
				if name == "--metrics.addr" || name == "--metrics.port" {
					t.Errorf("%s passes geth-only flag %q", chain, arg)
				}
			}
		})
	}
}

// The rendered config must be the file the node is started with.
func TestEVMConfigFileIsPassedToNode(t *testing.T) {
	for _, chain := range []chainsv1alpha2.Chain{
		chainsv1alpha2.ChainEthereumClassic,
		chainsv1alpha2.ChainCore,
		chainsv1alpha2.ChainBSC,
		chainsv1alpha2.ChainPolygon,
		chainsv1alpha2.ChainMetis,
		chainsv1alpha2.ChainArbitrum,
		chainsv1alpha2.ChainEverclear,
		chainsv1alpha2.ChainPlaynance,
		chainsv1alpha2.ChainGravityAlpha,
		chainsv1alpha2.ChainPlume,
	} {
		t.Run(string(chain), func(t *testing.T) {
			adapter, _ := adapters.Get(chain)
			filename, _, err := adapter.ConfigTemplate(chainsv1alpha2.ChainInstanceSpec{Chain: chain})
			if err != nil {
				t.Fatalf("ConfigTemplate: %v", err)
			}
			line := strings.Join(evmFlagsCommandLine(t, chain, chainsv1alpha2.NetworkMainnet), " ")
			if !strings.Contains(line, "/config/"+filename) {
				t.Errorf("%s does not pass /config/%s to the node: %s", chain, filename, line)
			}
		})
	}
}

// Gnosis and Fuse run Nethermind's built-in network config, which carries
// the chainspec and sync pivot, with the data on the data volume.
func TestNethermindBuiltInConfigs(t *testing.T) {
	for _, tc := range []struct {
		chain   chainsv1alpha2.Chain
		network chainsv1alpha2.Network
		config  string
	}{
		{chainsv1alpha2.ChainGnosis, chainsv1alpha2.NetworkMainnet, "gnosis"},
		{chainsv1alpha2.ChainGnosis, chainsv1alpha2.NetworkTestnet, "chiado"},
		{chainsv1alpha2.ChainFuse, chainsv1alpha2.NetworkMainnet, "fuse"},
		{chainsv1alpha2.ChainFuse, chainsv1alpha2.NetworkTestnet, "spark"},
	} {
		t.Run(string(tc.chain)+"/"+string(tc.network), func(t *testing.T) {
			line := evmFlagsCommandLine(t, tc.chain, tc.network)
			joined := strings.Join(line, " ")
			for _, want := range []string{
				"--config " + tc.config,
				"--datadir /data",
				"--JsonRpc.Host 0.0.0.0",
				"--Metrics.Enabled true",
				"--Metrics.ExposePort 6060",
			} {
				if !strings.Contains(joined, want) {
					t.Errorf("args lack %q: %s", want, joined)
				}
			}
		})
	}
}

// BSC and Core start from the genesis built into their geth instead of a
// downloaded file.
func TestGethForksUseBuiltInGenesis(t *testing.T) {
	for _, tc := range []struct {
		chain   chainsv1alpha2.Chain
		network chainsv1alpha2.Network
		flag    string
	}{
		{chainsv1alpha2.ChainBSC, chainsv1alpha2.NetworkMainnet, "--mainnet"},
		{chainsv1alpha2.ChainBSC, chainsv1alpha2.NetworkTestnet, "--chapel"},
		{chainsv1alpha2.ChainCore, chainsv1alpha2.NetworkMainnet, "--mainnet"},
		{chainsv1alpha2.ChainCore, chainsv1alpha2.NetworkTestnet, "--pigeon"},
		{chainsv1alpha2.ChainEthereumClassic, chainsv1alpha2.NetworkMainnet, "--classic"},
		{chainsv1alpha2.ChainEthereumClassic, chainsv1alpha2.NetworkTestnet, "--mordor"},
	} {
		t.Run(string(tc.chain)+"/"+string(tc.network), func(t *testing.T) {
			line := evmFlagsCommandLine(t, tc.chain, tc.network)
			if !slices.Contains(line, tc.flag) {
				t.Errorf("command line lacks %s: %v", tc.flag, line)
			}
			if slices.ContainsFunc(line, func(s string) bool { return strings.Contains(s, "wget") }) {
				t.Errorf("command line still downloads files: %v", line)
			}
		})
	}
}

// The Nitro config is valid JSON, keeps chain state on the data volume and
// exposes metrics on all interfaces.
func TestNitroConfig(t *testing.T) {
	for _, tc := range []struct {
		chain     chainsv1alpha2.Chain
		network   chainsv1alpha2.Network
		chainID   float64
		chainInfo bool
	}{
		{chainsv1alpha2.ChainArbitrum, chainsv1alpha2.NetworkMainnet, 42161, false},
		{chainsv1alpha2.ChainArbitrum, chainsv1alpha2.NetworkTestnet, 421614, false},
		{chainsv1alpha2.ChainEverclear, chainsv1alpha2.NetworkMainnet, 25327, false},
		{chainsv1alpha2.ChainPlaynance, chainsv1alpha2.NetworkMainnet, 1829, false},
		{chainsv1alpha2.ChainGravityAlpha, chainsv1alpha2.NetworkMainnet, 1625, true},
		{chainsv1alpha2.ChainPlume, chainsv1alpha2.NetworkMainnet, 98866, true},
		{chainsv1alpha2.ChainPlume, chainsv1alpha2.NetworkTestnet, 98867, true},
	} {
		t.Run(string(tc.chain)+"/"+string(tc.network), func(t *testing.T) {
			adapter, _ := adapters.Get(tc.chain)
			_, content, err := adapter.ConfigTemplate(chainsv1alpha2.ChainInstanceSpec{Chain: tc.chain, Network: tc.network})
			if err != nil {
				t.Fatalf("ConfigTemplate: %v", err)
			}
			var cfg struct {
				Chain struct {
					ID       float64 `json:"id"`
					InfoJSON string  `json:"info-json"`
				} `json:"chain"`
				Persistent struct {
					GlobalConfig string `json:"global-config"`
				} `json:"persistent"`
				Node struct {
					Staker struct {
						Enable *bool `json:"enable"`
					} `json:"staker"`
				} `json:"node"`
				Metrics       bool `json:"metrics"`
				MetricsServer struct {
					Addr string `json:"addr"`
				} `json:"metrics-server"`
			}
			if err := json.Unmarshal([]byte(content), &cfg); err != nil {
				t.Fatalf("config is not JSON: %v", err)
			}
			if cfg.Chain.ID != tc.chainID {
				t.Errorf("chain.id = %v, want %v", cfg.Chain.ID, tc.chainID)
			}
			if cfg.Persistent.GlobalConfig != "/data" {
				t.Errorf("persistent.global-config = %q, want /data", cfg.Persistent.GlobalConfig)
			}
			if !cfg.Metrics || cfg.MetricsServer.Addr != "0.0.0.0" {
				t.Errorf("metrics = %v on %q, want true on 0.0.0.0", cfg.Metrics, cfg.MetricsServer.Addr)
			}
			if e := cfg.Node.Staker.Enable; e == nil || *e {
				t.Error("node.staker.enable is not false; RPC nodes must not run the watchtower validator")
			}
			if !tc.chainInfo {
				return
			}
			var info []struct {
				ChainID uint64 `json:"chain-id"`
			}
			if err := json.Unmarshal([]byte(cfg.Chain.InfoJSON), &info); err != nil {
				t.Fatalf("chain.info-json is not JSON: %v", err)
			}
			if len(info) != 1 || float64(info[0].ChainID) != tc.chainID {
				t.Errorf("chain.info-json describes %+v, want chain %v", info, tc.chainID)
			}
		})
	}
}

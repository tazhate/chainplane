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
	"regexp"
	"slices"
	"strings"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// nodeCLI is the rendered entrypoint, args and env names of a chain's main
// container.
type nodeCLI struct {
	command []string
	args    []string
	env     map[string]bool
}

func renderNodeCLI(t *testing.T, chain chainsv1alpha2.Chain, network chainsv1alpha2.Network) nodeCLI {
	t.Helper()
	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chain, Network: network}
	a := adapters.MustGet(chain)
	var c nodeCLI
	if cp, ok := a.(adapters.ContainerCommandProvider); ok {
		c.command = cp.ContainerCommand(spec)
	}
	if ap, ok := a.(adapters.ContainerArgsProvider); ok {
		c.args = ap.ContainerArgs(spec)
	}
	c.env = map[string]bool{}
	if ep, ok := a.(adapters.ContainerEnvProvider); ok {
		for _, e := range ep.ContainerEnv(spec) {
			c.env[e.Name] = true
		}
	}
	return c
}

// flagValue returns the argument after flag, or "" when flag is absent.
func (c nodeCLI) flagValue(flag string) string {
	i := slices.Index(c.args, flag)
	if i < 0 || i+1 >= len(c.args) {
		return ""
	}
	return c.args[i+1]
}

func (c nodeCLI) flagValues(flag string) []string {
	var out []string
	for i, a := range c.args {
		if a == flag && i+1 < len(c.args) {
			out = append(out, c.args[i+1])
		}
	}
	return out
}

var envRefPattern = regexp.MustCompile(`\$\(([A-Z0-9_]+)\)`)

// requireEnvRefsDefined fails when an arg expands $(VAR) the adapter does not
// set: Kubernetes would pass the literal string through.
func (c nodeCLI) requireEnvRefsDefined(t *testing.T) {
	t.Helper()
	for _, a := range c.args {
		for _, m := range envRefPattern.FindAllStringSubmatch(a, -1) {
			if !c.env[m[1]] {
				t.Errorf("arg %q references $(%s), not set by ContainerEnv", a, m[1])
			}
		}
	}
}

// TestSubstrateNoRemovedFlags guards against flags polkadot-sdk 1.x rejects
// at startup: the separate WebSocket server (merged into --rpc-port) and
// --config, which moonbeam never had.
func TestSubstrateNoRemovedFlags(t *testing.T) {
	removed := []string{"--ws-port", "--ws-external", "--unsafe-ws-external", "--ws-max-connections", "--config"}
	for _, chain := range []chainsv1alpha2.Chain{
		chainsv1alpha2.ChainPolkadot,
		chainsv1alpha2.ChainKusama,
		chainsv1alpha2.ChainMoonbeam,
		chainsv1alpha2.ChainMoonriver,
	} {
		t.Run(string(chain), func(t *testing.T) {
			c := renderNodeCLI(t, chain, chainsv1alpha2.NetworkMainnet)
			for _, f := range removed {
				if slices.Contains(c.args, f) {
					t.Errorf("args contain removed flag %s: %v", f, c.args)
				}
			}
			if got := c.flagValue("--rpc-port"); got != "9944" {
				t.Errorf("--rpc-port = %q, want 9944 (serves HTTP and WS)", got)
			}
			ports := adapters.MustGet(chain).ContainerPorts(chainsv1alpha2.ChainInstanceSpec{Chain: chain})
			for _, p := range ports {
				if p.ContainerPort == 9945 {
					t.Errorf("container port %s 9945 has no listener", p.Name)
				}
			}
		})
	}
}

func TestMoonbeamChainSpec(t *testing.T) {
	tests := []struct {
		chain   chainsv1alpha2.Chain
		network chainsv1alpha2.Network
		want    string
	}{
		{chainsv1alpha2.ChainMoonbeam, chainsv1alpha2.NetworkMainnet, "moonbeam"},
		{chainsv1alpha2.ChainMoonriver, chainsv1alpha2.NetworkMainnet, "moonriver"},
		{chainsv1alpha2.ChainMoonbeam, chainsv1alpha2.NetworkTestnet, "moonbase-alpha"},
		{chainsv1alpha2.ChainMoonriver, chainsv1alpha2.NetworkTestnet, "moonbase-alpha"},
	}
	for _, tt := range tests {
		t.Run(string(tt.chain)+"/"+string(tt.network), func(t *testing.T) {
			c := renderNodeCLI(t, tt.chain, tt.network)
			if got := c.flagValue("--chain"); got != tt.want {
				t.Errorf("--chain = %q, want %q", got, tt.want)
			}
			if got := c.flagValue("--base-path"); got != "/data" {
				t.Errorf("--base-path = %q, want /data", got)
			}
			// A "--" would send spec.extraArgs to the embedded relay chain.
			if slices.Contains(c.args, "--") {
				t.Errorf("args must not open the relay chain section: %v", c.args)
			}
		})
	}
}

// TestLighthouseBeaconNode: the sigp/lighthouse image has no entrypoint, so a
// container without command and args exits at once.
func TestLighthouseBeaconNode(t *testing.T) {
	tests := []struct {
		chain   chainsv1alpha2.Chain
		network chainsv1alpha2.Network
		want    string
	}{
		{chainsv1alpha2.ChainEthereumBeacon, chainsv1alpha2.NetworkMainnet, "mainnet"},
		{chainsv1alpha2.ChainEthereumBeacon, chainsv1alpha2.NetworkTestnet, "sepolia"},
		{chainsv1alpha2.ChainGnosisBeacon, chainsv1alpha2.NetworkMainnet, "gnosis"},
		{chainsv1alpha2.ChainGnosisBeacon, chainsv1alpha2.NetworkTestnet, "chiado"},
	}
	for _, tt := range tests {
		t.Run(string(tt.chain)+"/"+string(tt.network), func(t *testing.T) {
			c := renderNodeCLI(t, tt.chain, tt.network)
			if !slices.Equal(c.command, []string{"lighthouse"}) {
				t.Errorf("command = %v, want [lighthouse]", c.command)
			}
			if len(c.args) == 0 || c.args[0] != "bn" {
				t.Fatalf("args must start with the bn subcommand: %v", c.args)
			}
			if got := c.flagValue("--network"); got != tt.want {
				t.Errorf("--network = %q, want %q", got, tt.want)
			}
			for _, f := range []string{"--datadir", "--http-address", "--execution-endpoint", "--execution-jwt", "--checkpoint-sync-url"} {
				if c.flagValue(f) == "" {
					t.Errorf("missing %s: %v", f, c.args)
				}
			}
			if got := c.flagValue("--http-address"); got != "0.0.0.0" {
				t.Errorf("--http-address = %q, want 0.0.0.0 for the health check", got)
			}
			c.requireEnvRefsDefined(t)
		})
	}
}

// TestStarknetJunoFlags: Juno refuses --p2p on mainnet and exits without an
// L1 node unless L1 verification is disabled.
func TestStarknetJunoFlags(t *testing.T) {
	c := renderNodeCLI(t, chainsv1alpha2.ChainStarknet, chainsv1alpha2.NetworkMainnet)
	for _, a := range c.args {
		if strings.HasPrefix(a, "--p2p") {
			t.Errorf("args contain %s, which juno rejects on mainnet", a)
		}
	}
	if c.flagValue("--eth-node") == "" && !slices.Contains(c.args, "--disable-l1-verification") {
		t.Error("juno needs --eth-node or --disable-l1-verification")
	}
	if got := c.flagValue("--metrics-host"); got != "0.0.0.0" {
		t.Errorf("--metrics-host = %q, want 0.0.0.0 (juno defaults to localhost)", got)
	}
	c.requireEnvRefsDefined(t)
}

// TestSolanaJoinsPublicCluster: the agave image entrypoint (solana-run.sh)
// starts a private dev cluster, so the adapter must replace it and point the
// validator at the public cluster.
func TestSolanaJoinsPublicCluster(t *testing.T) {
	tests := []struct {
		network     chainsv1alpha2.Network
		genesisHash string
		entrypoint  string
	}{
		{chainsv1alpha2.NetworkMainnet, "5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d", "entrypoint.mainnet-beta.solana.com:8001"},
		{chainsv1alpha2.NetworkTestnet, "4uhcVJyU9pJkvQyS88uRDiswHXSCkY3zQawwpjk2NsNY", "entrypoint.testnet.solana.com:8001"},
	}
	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			c := renderNodeCLI(t, chainsv1alpha2.ChainSolana, tt.network)
			if len(c.command) == 0 || !strings.Contains(strings.Join(c.command, " "), "exec agave-validator --identity") {
				t.Errorf("command must exec agave-validator with an identity: %v", c.command)
			}
			if got := c.flagValue("--expected-genesis-hash"); got != tt.genesisHash {
				t.Errorf("--expected-genesis-hash = %q, want %q", got, tt.genesisHash)
			}
			if !slices.Contains(c.flagValues("--entrypoint"), tt.entrypoint) {
				t.Errorf("--entrypoint values %v lack %s", c.flagValues("--entrypoint"), tt.entrypoint)
			}
			known := c.flagValues("--known-validator")
			if len(known) == 0 {
				t.Error("no --known-validator")
			}
			for _, v := range known {
				if strings.Contains(v, ":") {
					t.Errorf("--known-validator %q is a host:port, want an identity pubkey", v)
				}
			}
			if !slices.Contains(c.args, "--no-voting") {
				t.Error("RPC node must run with --no-voting")
			}
		})
	}
}

// TestSuiNoDeadArchiveConfig: state sync ignores object-store archive entries
// and rejects checkpoints.mainnet.sui.io as an archive, so neither may stand
// in for a working fallback.
func TestSuiNoDeadArchiveConfig(t *testing.T) {
	_, cfg, err := adapters.MustGet(chainsv1alpha2.ChainSui).ConfigTemplate(chainsv1alpha2.ChainInstanceSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cfg, "object-store-config") {
		t.Error("object-store-config archive entries are ignored by state sync")
	}
	if strings.Contains(cfg, `ingestion-url: "https://checkpoints.mainnet.sui.io"`) {
		t.Error("state sync rejects checkpoints.mainnet.sui.io as an archive")
	}
}

// TestSolanaExporterPinned: the former nordstroem/solana-exporter:latest
// image does not exist, so the pod never started.
func TestSolanaExporterPinned(t *testing.T) {
	sp := adapters.MustGet(chainsv1alpha2.ChainSolana).(adapters.SidecarProvider)
	for _, c := range sp.Sidecars(chainsv1alpha2.ChainInstanceSpec{Chain: chainsv1alpha2.ChainSolana}) {
		if c.Image == "" || strings.HasSuffix(c.Image, ":latest") || !strings.Contains(c.Image, ":") {
			t.Errorf("sidecar %s image %q must be a pinned tag", c.Name, c.Image)
		}
	}
}

/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package adapters_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// ethSpec is an ethereum or ethereum-archive spec for one client and network.
func ethSpec(chain chainsv1alpha2.Chain, client string, network chainsv1alpha2.Network) chainsv1alpha2.ChainInstanceSpec {
	nodeType := chainsv1alpha2.NodeTypeRPC
	if chain == chainsv1alpha2.ChainEthereumArchive {
		nodeType = chainsv1alpha2.NodeTypeArchive
	}
	return chainsv1alpha2.ChainInstanceSpec{Chain: chain, Network: network, Client: client, NodeType: nodeType}
}

func ethArgs(t *testing.T, spec chainsv1alpha2.ChainInstanceSpec) []string {
	t.Helper()
	adapter := adapters.MustGet(spec.Chain)
	ap, ok := adapter.(adapters.ContainerArgsProvider)
	if !ok {
		t.Fatal("ethereum adapter has no ContainerArgs")
	}
	return ap.ContainerArgs(spec)
}

// hasPair reports whether flag is followed by value in args.
func hasPair(args []string, flag, value string) bool {
	i := slices.Index(args, flag)
	return i >= 0 && i+1 < len(args) && args[i+1] == value
}

// TestEthereumClientsUseDataVolume is the regression test for clients that
// never saw the operator's settings and kept their chain in the container
// filesystem: every client must put its datadir on /data.
func TestEthereumClientsUseDataVolume(t *testing.T) {
	for _, chain := range []chainsv1alpha2.Chain{chainsv1alpha2.ChainEthereum, chainsv1alpha2.ChainEthereumArchive} {
		for _, client := range []string{"", "nethermind", "geth", "reth", "erigon"} {
			for _, network := range []chainsv1alpha2.Network{chainsv1alpha2.NetworkMainnet, chainsv1alpha2.NetworkTestnet} {
				spec := ethSpec(chain, client, network)
				t.Run(string(chain)+"/"+client+"/"+string(network), func(t *testing.T) {
					args := ethArgs(t, spec)
					file, content, err := adapters.MustGet(chain).ConfigTemplate(spec)
					if err != nil {
						t.Fatal(err)
					}
					if client == "geth" {
						if !hasPair(args, "--config", "/config/"+file) {
							t.Errorf("geth does not read the rendered %s: %q", file, args)
						}
						if !strings.Contains(content, `DataDir = "/data"`) {
							t.Errorf("geth config has no DataDir /data:\n%s", content)
						}
						return
					}
					if !hasPair(args, "--datadir", "/data") {
						t.Errorf("no --datadir /data: %q", args)
					}
				})
			}
		}
	}
}

// TestEthereumNoForeignFlags keeps flags of one client away from another:
// Nethermind and reth exit on go-ethereum flags they do not know.
func TestEthereumNoForeignFlags(t *testing.T) {
	gethOnly := []string{"--config", "--sepolia", "--syncmode", "--gcmode", "--metrics.addr", "--metrics.port", "--http.vhosts"}
	nethermindFlag := regexp.MustCompile(`^--[A-Z][A-Za-z]*\.[A-Za-z0-9]+$`)
	for _, network := range []chainsv1alpha2.Network{chainsv1alpha2.NetworkMainnet, chainsv1alpha2.NetworkTestnet} {
		for _, client := range []string{"nethermind", "reth", "erigon"} {
			args := ethArgs(t, ethSpec(chainsv1alpha2.ChainEthereum, client, network))
			for _, a := range args {
				if !strings.HasPrefix(a, "--") {
					continue
				}
				switch client {
				case "nethermind":
					if a != "--config" && a != "--datadir" && !nethermindFlag.MatchString(a) {
						t.Errorf("nethermind/%s: %s is not a Nethermind flag", network, a)
					}
				case "reth":
					if slices.Contains(gethOnly, a) {
						t.Errorf("reth/%s: geth flag %s", network, a)
					}
				case "erigon":
					// Erigon shares --metrics.addr/--metrics.port and --http.vhosts with geth.
					if a == "--config" || a == "--sepolia" || a == "--syncmode" || a == "--gcmode" {
						t.Errorf("erigon/%s: geth flag %s", network, a)
					}
				}
			}
		}
	}
	if args := ethArgs(t, ethSpec(chainsv1alpha2.ChainEthereum, "reth", chainsv1alpha2.NetworkMainnet)); args[0] != "node" {
		t.Errorf("reth args must start with the node subcommand: %q", args)
	}
}

func TestEthereumNetworkAndArchive(t *testing.T) {
	mainnet, testnet := chainsv1alpha2.NetworkMainnet, chainsv1alpha2.NetworkTestnet
	eth, archive := chainsv1alpha2.ChainEthereum, chainsv1alpha2.ChainEthereumArchive
	tests := []struct {
		spec    chainsv1alpha2.ChainInstanceSpec
		want    [][2]string // flag/value pairs, value "" for a bare flag
		notWant []string
	}{
		{ethSpec(eth, "nethermind", mainnet), [][2]string{{"--config", "mainnet"}}, nil},
		{ethSpec(eth, "nethermind", testnet), [][2]string{{"--config", "sepolia"}}, nil},
		{ethSpec(archive, "nethermind", mainnet), [][2]string{{"--config", "mainnet_archive"}}, nil},
		{ethSpec(archive, "nethermind", testnet), [][2]string{{"--config", "sepolia_archive"}}, nil},
		{ethSpec(eth, "unknown", mainnet), [][2]string{{"--config", "mainnet"}, {"--datadir", "/data"}}, nil},
		{ethSpec(eth, "geth", mainnet), nil, []string{"--sepolia"}},
		{ethSpec(eth, "geth", testnet), [][2]string{{"--sepolia", ""}}, nil},
		{ethSpec(eth, "reth", mainnet), [][2]string{{"--chain", "mainnet"}, {"--full", ""}}, nil},
		{ethSpec(archive, "reth", testnet), [][2]string{{"--chain", "sepolia"}}, []string{"--full"}},
		{ethSpec(eth, "erigon", mainnet), [][2]string{{"--chain", "mainnet"}, {"--prune", "hrtc"}}, nil},
		{ethSpec(archive, "erigon", testnet), [][2]string{{"--chain", "sepolia"}}, []string{"--prune"}},
	}
	for _, tt := range tests {
		args := ethArgs(t, tt.spec)
		for _, w := range tt.want {
			if (w[1] == "" && !slices.Contains(args, w[0])) || (w[1] != "" && !hasPair(args, w[0], w[1])) {
				t.Errorf("%s/%s/%s: missing %s %s in %q", tt.spec.Chain, tt.spec.Client, tt.spec.Network, w[0], w[1], args)
			}
		}
		for _, n := range tt.notWant {
			if slices.Contains(args, n) {
				t.Errorf("%s/%s/%s: unexpected %s in %q", tt.spec.Chain, tt.spec.Client, tt.spec.Network, n, args)
			}
		}
	}
}

// TestEthereumGethConfig checks the TOML against what geth v1.17 decodes:
// durations are integers (a "30s" string is a fatal error at start) and
// archive mode is full sync without pruning.
func TestEthereumGethConfig(t *testing.T) {
	adapter := adapters.MustGet(chainsv1alpha2.ChainEthereum)
	file, full, err := adapter.ConfigTemplate(ethSpec(chainsv1alpha2.ChainEthereum, "geth", chainsv1alpha2.NetworkMainnet))
	if err != nil {
		t.Fatal(err)
	}
	if file != "config.toml" {
		t.Errorf("geth config file = %q", file)
	}
	if regexp.MustCompile(`Timeout = "`).MatchString(full) {
		t.Errorf("duration as a string, geth wants nanoseconds:\n%s", full)
	}
	for _, want := range []string{`SyncMode = "snap"`, `AuthAddr = "0.0.0.0"`, `JWTSecret = "/data/jwt.hex"`, `HTTPHost = "0.0.0.0"`} {
		if !strings.Contains(full, want) {
			t.Errorf("full node config lacks %s:\n%s", want, full)
		}
	}
	_, arch, err := adapter.ConfigTemplate(ethSpec(chainsv1alpha2.ChainEthereumArchive, "geth", chainsv1alpha2.NetworkMainnet))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`SyncMode = "full"`, `NoPruning = true`} {
		if !strings.Contains(arch, want) {
			t.Errorf("archive config lacks %s:\n%s", want, arch)
		}
	}
}

func TestEthereumConfigFiles(t *testing.T) {
	adapter := adapters.MustGet(chainsv1alpha2.ChainEthereum)
	for client, want := range map[string]string{
		"geth": "config.toml", "nethermind": "nethermind.json", "": "nethermind.json", "reth": "", "erigon": "",
	} {
		file, content, err := adapter.ConfigTemplate(ethSpec(chainsv1alpha2.ChainEthereum, client, chainsv1alpha2.NetworkMainnet))
		if err != nil {
			t.Fatal(err)
		}
		if file != want || (want != "" && content == "") || (want == "" && content != "") {
			t.Errorf("client %q: file %q (%d bytes), want %q", client, file, len(content), want)
		}
	}
}

// TestEthereumPorts checks the Engine API port the beacon node dials and that
// the metrics port is the one the client serves.
func TestEthereumPorts(t *testing.T) {
	adapter := adapters.MustGet(chainsv1alpha2.ChainEthereum)
	for client, metrics := range map[string]int32{"geth": 6060, "erigon": 6060, "reth": 9001, "nethermind": 9091, "": 9091} {
		ports := adapter.ContainerPorts(ethSpec(chainsv1alpha2.ChainEthereum, client, chainsv1alpha2.NetworkMainnet))
		byName := map[string]int32{}
		for _, p := range ports {
			if p.Protocol != corev1.ProtocolUDP {
				byName[p.Name] = p.ContainerPort
			}
		}
		if byName["rpc"] != 8545 || byName["engine"] != 8551 || byName["metrics"] != metrics {
			t.Errorf("client %q: ports %v, want rpc 8545, engine 8551, metrics %d", client, byName, metrics)
		}
	}
}

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
	"strings"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"

	// Register all adapters via init()
	_ "github.com/tazhate/chainplane/internal/adapters"
)

// allChains lists every chain the operator supports — 100 total.
var allChains = []chainsv1alpha2.Chain{
	chainsv1alpha2.ChainEthereum,
	chainsv1alpha2.ChainEthereumArchive,
	chainsv1alpha2.ChainBitcoin,
	chainsv1alpha2.ChainSolana,
	chainsv1alpha2.ChainBSC,
	chainsv1alpha2.ChainTRON,
	chainsv1alpha2.ChainPolygon,
	chainsv1alpha2.ChainAvalanche,
	chainsv1alpha2.ChainLitecoin,
	chainsv1alpha2.ChainXRP,
	chainsv1alpha2.ChainStellar,
	chainsv1alpha2.ChainDash,
	chainsv1alpha2.ChainTON,
	chainsv1alpha2.ChainCosmos,
	chainsv1alpha2.ChainNear,
	chainsv1alpha2.ChainSui,
	chainsv1alpha2.ChainAptos,
	chainsv1alpha2.ChainCardano,
	chainsv1alpha2.ChainArbitrum,
	chainsv1alpha2.ChainOptimism,
	chainsv1alpha2.ChainBase,
	chainsv1alpha2.ChainBerachain,
	chainsv1alpha2.ChainCronos,
	chainsv1alpha2.ChainRonin,
	chainsv1alpha2.ChainCelo,
	chainsv1alpha2.ChainFantom,
	chainsv1alpha2.ChainGnosis,
	chainsv1alpha2.ChainMantle,
	chainsv1alpha2.ChainBlast,
	chainsv1alpha2.ChainMode,
	chainsv1alpha2.ChainZora,
	chainsv1alpha2.ChainTaiko,
	chainsv1alpha2.ChainZkSync,
	chainsv1alpha2.ChainLinea,
	chainsv1alpha2.ChainScroll,
	chainsv1alpha2.ChainDogecoin,
	chainsv1alpha2.ChainOsmosis,
	chainsv1alpha2.ChainSei,
	chainsv1alpha2.ChainEvmos,
	chainsv1alpha2.ChainKava,
	chainsv1alpha2.ChainPolkadot,
	chainsv1alpha2.ChainStarknet,
	chainsv1alpha2.ChainFilecoin,
	chainsv1alpha2.ChainMoonbeam,
	chainsv1alpha2.ChainMoonriver,
	chainsv1alpha2.ChainPolygonZkEVM,
	chainsv1alpha2.ChainMantaPacific,
	chainsv1alpha2.ChainMetis,
	chainsv1alpha2.ChainFraxtal,
	chainsv1alpha2.ChainLisk,
	chainsv1alpha2.ChainKroma,
	chainsv1alpha2.ChainBob,
	chainsv1alpha2.ChainBobaEth,
	chainsv1alpha2.ChainSoneium,
	chainsv1alpha2.ChainSwell,
	chainsv1alpha2.ChainSuperseed,
	chainsv1alpha2.ChainInk,
	chainsv1alpha2.ChainMorph,
	chainsv1alpha2.ChainAbstract,
	chainsv1alpha2.ChainMegaETH,
	chainsv1alpha2.ChainZeroNetwork,
	chainsv1alpha2.ChainZircuit,
	chainsv1alpha2.ChainImmutableZkEVM,
	chainsv1alpha2.ChainWorldchain,
	chainsv1alpha2.ChainUnichain,
	chainsv1alpha2.ChainLens,
	chainsv1alpha2.ChainPlume,
	chainsv1alpha2.ChainHemi,
	chainsv1alpha2.ChainAxelar,
	chainsv1alpha2.ChainDymension,
	chainsv1alpha2.ChainAurora,
	chainsv1alpha2.ChainHarmony,
	chainsv1alpha2.ChainRootstock,
	chainsv1alpha2.ChainTelos,
	chainsv1alpha2.ChainKlaytn,
	chainsv1alpha2.ChainBitTorrent,
	chainsv1alpha2.ChainGravityAlpha,
	chainsv1alpha2.ChainMoca,
	chainsv1alpha2.ChainEverclear,
	chainsv1alpha2.ChainDoma,
	chainsv1alpha2.ChainShibarium,
	chainsv1alpha2.ChainCore,
	chainsv1alpha2.ChainHaqq,
	chainsv1alpha2.ChainHashKey,
	chainsv1alpha2.ChainEthereumClassic,
	chainsv1alpha2.ChainCronosZkEVM,
	chainsv1alpha2.ChainSonic,
	chainsv1alpha2.ChainGoat,
	chainsv1alpha2.ChainKatana,
	chainsv1alpha2.ChainMezo,
	chainsv1alpha2.ChainPlasma,
	chainsv1alpha2.ChainPlaynance,
	chainsv1alpha2.ChainOpBNB,
	chainsv1alpha2.ChainFuse,
	chainsv1alpha2.ChainThundercore,
	chainsv1alpha2.ChainWemix,
	chainsv1alpha2.ChainViction,
	chainsv1alpha2.ChainEthereumBeacon,
	chainsv1alpha2.ChainGnosisBeacon,
	chainsv1alpha2.ChainKusama,
	chainsv1alpha2.ChainHyperliquid,
	chainsv1alpha2.ChainMonad,
}

// ---------------------------------------------------------------------------
// Registry: all adapters are registered
// ---------------------------------------------------------------------------

func TestAllAdaptersRegistered(t *testing.T) {
	if len(allChains) != 102 {
		t.Fatalf("expected 102 chains in allChains, got %d", len(allChains))
	}
	for _, chain := range allChains {
		_, ok := adapters.Get(chain)
		if !ok {
			t.Errorf("adapter not registered for chain: %s", chain)
		}
	}
}

func TestRegistryCount(t *testing.T) {
	registered := 0
	for _, chain := range allChains {
		if _, ok := adapters.Get(chain); ok {
			registered++
		}
	}
	if registered != 102 {
		t.Errorf("expected 102 registered adapters, got %d", registered)
	}
}

func TestGetUnknownChainReturnsNil(t *testing.T) {
	_, ok := adapters.Get(chainsv1alpha2.Chain("nonexistent"))
	if ok {
		t.Error("expected Get to return false for unknown chain")
	}
}

// ---------------------------------------------------------------------------
// DefaultImage: every adapter returns non-empty image for default client
// ---------------------------------------------------------------------------

func TestAllAdaptersDefaultImage(t *testing.T) {
	for _, chain := range allChains {
		chain := chain
		t.Run(string(chain), func(t *testing.T) {
			adapter, ok := adapters.Get(chain)
			if !ok {
				t.Fatalf("adapter not registered for chain: %s", chain)
			}
			img := adapter.DefaultImage("")
			if img == "" {
				t.Errorf("chain %s: DefaultImage returned empty string", chain)
			}
			// Image should contain a slash (registry/image format)
			if !strings.Contains(img, "/") {
				t.Errorf("chain %s: DefaultImage %q does not look like a valid container image", chain, img)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ConfigTemplate: every adapter returns non-empty filename and content,
// unless the chain is configured via env vars / CLI flags (no config file).
// ---------------------------------------------------------------------------

// noConfigChains lists chains that legitimately return an empty ConfigTemplate
// because they are configured via environment variables and CLI flags rather
// than a mounted config file. ZK Stack external nodes and Arbitrum Orbit/Nitro
// nodes fall into this category.
var noConfigChains = map[chainsv1alpha2.Chain]bool{
	chainsv1alpha2.ChainLens:         true, // ZK Stack external node
	chainsv1alpha2.ChainAbstract:     true, // ZK Stack external node
	chainsv1alpha2.ChainZeroNetwork:  true, // ZK Stack external node
	chainsv1alpha2.ChainCronosZkEVM:  true, // ZK Stack external node
	chainsv1alpha2.ChainEverclear:    true, // Arbitrum Orbit (AnyTrust)
	chainsv1alpha2.ChainPlaynance:    true, // Arbitrum Orbit L3
	chainsv1alpha2.ChainGravityAlpha: true, // Arbitrum Nitro + Celestia DA
	chainsv1alpha2.ChainPlume:        true, // Arbitrum Nitro
}

func TestAllAdaptersConfigTemplate(t *testing.T) {
	for _, network := range []chainsv1alpha2.Network{
		chainsv1alpha2.NetworkMainnet,
		chainsv1alpha2.NetworkTestnet,
	} {
		for _, chain := range allChains {
			chain := chain
			t.Run(string(chain)+"/"+string(network), func(t *testing.T) {
				adapter, ok := adapters.Get(chain)
				if !ok {
					t.Fatalf("adapter not registered for chain: %s", chain)
				}
				spec := chainsv1alpha2.ChainInstanceSpec{
					Chain:    chain,
					Network:  network,
					NodeType: chainsv1alpha2.NodeTypeRPC,
				}
				filename, content, err := adapter.ConfigTemplate(spec)
				if err != nil {
					t.Fatalf("ConfigTemplate error: %v", err)
				}
				if noConfigChains[chain] {
					return // chains without config files legitimately return empty
				}
				if filename == "" {
					t.Error("ConfigTemplate returned empty filename")
				}
				if content == "" {
					t.Error("ConfigTemplate returned empty content")
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// ContainerPorts: every adapter returns at least one port
// ---------------------------------------------------------------------------

func TestAllAdaptersContainerPorts(t *testing.T) {
	for _, chain := range allChains {
		chain := chain
		t.Run(string(chain), func(t *testing.T) {
			adapter, ok := adapters.Get(chain)
			if !ok {
				t.Fatalf("adapter not registered for chain: %s", chain)
			}
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain: chain,
				RPC:   chainsv1alpha2.RPCSpec{Enabled: true, Port: 8545},
			}
			ports := adapter.ContainerPorts(spec)
			if len(ports) == 0 {
				t.Errorf("chain %s: no container ports returned", chain)
			}
			// Every port should have a name and positive port number
			for i, p := range ports {
				if p.Name == "" {
					t.Errorf("chain %s: port[%d] has empty name", chain, i)
				}
				if p.ContainerPort <= 0 {
					t.Errorf("chain %s: port[%d] has non-positive container port: %d", chain, i, p.ContainerPort)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LivenessProbe: every adapter returns non-nil probe
// ---------------------------------------------------------------------------

func TestAllAdaptersLivenessProbe(t *testing.T) {
	for _, chain := range allChains {
		chain := chain
		t.Run(string(chain), func(t *testing.T) {
			adapter, ok := adapters.Get(chain)
			if !ok {
				t.Fatalf("adapter not registered for chain: %s", chain)
			}
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain: chain,
				RPC:   chainsv1alpha2.RPCSpec{Enabled: true, Port: 8545},
			}
			probe := adapter.LivenessProbe(spec)
			if probe == nil {
				t.Fatalf("chain %s: nil liveness probe", chain)
			}
			// Probe should have a handler configured
			if probe.ProbeHandler.TCPSocket == nil &&
				probe.ProbeHandler.HTTPGet == nil &&
				probe.ProbeHandler.Exec == nil &&
				probe.ProbeHandler.GRPC == nil {
				t.Errorf("chain %s: liveness probe has no handler", chain)
			}
			if probe.PeriodSeconds <= 0 {
				t.Errorf("chain %s: liveness probe PeriodSeconds should be positive", chain)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NodeSelector: every adapter returns non-nil map for all node groups
// ---------------------------------------------------------------------------

func TestAllAdaptersNodeSelector(t *testing.T) {
	// Specialised groups must return non-empty selectors; generic groups may return nil.
	specialised := []chainsv1alpha2.NodeGroup{
		chainsv1alpha2.NodeGroupStorage,
		chainsv1alpha2.NodeGroupBlockchain,
	}
	generic := []chainsv1alpha2.NodeGroup{
		chainsv1alpha2.NodeGroupLight,
		chainsv1alpha2.NodeGroupMedium,
		chainsv1alpha2.NodeGroupHeavy,
		chainsv1alpha2.NodeGroupArchive,
	}

	for _, chain := range allChains {
		chain := chain
		t.Run(string(chain), func(t *testing.T) {
			adapter, ok := adapters.Get(chain)
			if !ok {
				t.Fatalf("adapter not registered for chain: %s", chain)
			}
			for _, g := range specialised {
				sel := adapter.NodeSelector(g)
				if len(sel) == 0 {
					t.Errorf("chain %s, group %s: expected non-empty selector for specialised group", chain, g)
				}
			}
			for _, g := range generic {
				// nil is acceptable — pods schedule on any node
				_ = adapter.NodeSelector(g)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Ethereum: test all 4 clients return distinct images
// ---------------------------------------------------------------------------

func TestEthereumClientImages(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainEthereum)
	if !ok {
		t.Fatal("ethereum adapter not registered")
	}

	clients := []struct {
		name           string
		expectedSubstr string
	}{
		{name: "geth", expectedSubstr: "client-go"},
		{name: "reth", expectedSubstr: "reth"},
		{name: "erigon", expectedSubstr: "erigon"},
		{name: "nethermind", expectedSubstr: "nethermind"},
		{name: "", expectedSubstr: "nethermind"}, // default
	}

	images := make(map[string]string)
	for _, tc := range clients {
		tc := tc
		t.Run("client="+tc.name, func(t *testing.T) {
			img := adapter.DefaultImage(tc.name)
			if img == "" {
				t.Fatalf("client %q: empty image", tc.name)
			}
			if !strings.Contains(strings.ToLower(img), tc.expectedSubstr) {
				t.Errorf("client %q: image %q does not contain %q", tc.name, img, tc.expectedSubstr)
			}
			if tc.name != "" {
				images[tc.name] = img
			}
		})
	}

	// Verify all 4 named clients have distinct images
	t.Run("distinct_images", func(t *testing.T) {
		seen := make(map[string]string)
		for client, img := range images {
			if prev, exists := seen[img]; exists {
				t.Errorf("clients %q and %q share the same image: %s", prev, client, img)
			}
			seen[img] = client
		}
	})
}

func TestEthereumArchiveSharesAdapter(t *testing.T) {
	eth, ok1 := adapters.Get(chainsv1alpha2.ChainEthereum)
	archive, ok2 := adapters.Get(chainsv1alpha2.ChainEthereumArchive)
	if !ok1 || !ok2 {
		t.Fatal("ethereum or ethereum-archive adapter not registered")
	}
	// Both should return the same default image (same adapter type)
	if eth.DefaultImage("") != archive.DefaultImage("") {
		t.Error("ethereum and ethereum-archive should share the same adapter and default image")
	}
}

func TestEthereumConfigTemplateClients(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainEthereum)
	if !ok {
		t.Fatal("ethereum adapter not registered")
	}

	clients := []struct {
		name             string
		expectedFilename string
	}{
		{name: "geth", expectedFilename: "config.toml"},
		{name: "reth", expectedFilename: "reth.toml"},
		{name: "erigon", expectedFilename: "erigon.yaml"},
		{name: "nethermind", expectedFilename: "nethermind.json"},
	}

	for _, tc := range clients {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:    chainsv1alpha2.ChainEthereum,
				Network:  chainsv1alpha2.NetworkMainnet,
				NodeType: chainsv1alpha2.NodeTypeRPC,
				Client:   tc.name,
			}
			filename, content, err := adapter.ConfigTemplate(spec)
			if err != nil {
				t.Fatalf("ConfigTemplate error: %v", err)
			}
			if filename != tc.expectedFilename {
				t.Errorf("expected filename %q, got %q", tc.expectedFilename, filename)
			}
			if content == "" {
				t.Error("empty config content")
			}
		})
	}
}

func TestEthereumConfigTemplateNetworks(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainEthereum)
	if !ok {
		t.Fatal("ethereum adapter not registered")
	}

	for _, network := range []chainsv1alpha2.Network{
		chainsv1alpha2.NetworkMainnet,
		chainsv1alpha2.NetworkTestnet,
	} {
		t.Run(string(network), func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:    chainsv1alpha2.ChainEthereum,
				Network:  network,
				NodeType: chainsv1alpha2.NodeTypeRPC,
				Client:   "reth",
			}
			_, content, err := adapter.ConfigTemplate(spec)
			if err != nil {
				t.Fatalf("ConfigTemplate error: %v", err)
			}
			if network == chainsv1alpha2.NetworkTestnet {
				if !strings.Contains(content, "sepolia") {
					t.Error("testnet config should contain 'sepolia'")
				}
			} else {
				if !strings.Contains(content, "mainnet") {
					t.Error("mainnet config should contain 'mainnet'")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Bitcoin: config validation
// ---------------------------------------------------------------------------

func TestBitcoinMainnetConfig(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainBitcoin)
	if !ok {
		t.Fatal("bitcoin adapter not registered")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{
		Chain:   chainsv1alpha2.ChainBitcoin,
		Network: chainsv1alpha2.NetworkMainnet,
	}
	filename, content, err := adapter.ConfigTemplate(spec)
	if err != nil {
		t.Fatal(err)
	}
	if filename != "bitcoin.conf" {
		t.Errorf("expected filename 'bitcoin.conf', got %q", filename)
	}
	for _, expected := range []string{"server=1", "rpcallowip=", "rpcbind=0.0.0.0", "rpcport=8332", "txindex=1", "datadir=/data"} {
		if !strings.Contains(content, expected) {
			t.Errorf("mainnet config missing %q", expected)
		}
	}
	if strings.Contains(content, "testnet=1") {
		t.Error("mainnet config should not contain testnet=1")
	}
}

func TestBitcoinTestnetConfig(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainBitcoin)
	if !ok {
		t.Fatal("bitcoin adapter not registered")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{
		Chain:   chainsv1alpha2.ChainBitcoin,
		Network: chainsv1alpha2.NetworkTestnet,
	}
	filename, content, err := adapter.ConfigTemplate(spec)
	if err != nil {
		t.Fatal(err)
	}
	if filename != "bitcoin.conf" {
		t.Errorf("expected filename 'bitcoin.conf', got %q", filename)
	}
	if !strings.Contains(content, "testnet=1") {
		t.Errorf("testnet config missing 'testnet=1', got:\n%s", content)
	}
	if !strings.Contains(content, "rpcport=18332") {
		t.Error("testnet config should use rpcport=18332")
	}
	if !strings.Contains(content, "[test]") {
		t.Error("testnet config should contain [test] section")
	}
}

func TestBitcoinContainerPortsVaryByNetwork(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainBitcoin)
	if !ok {
		t.Fatal("bitcoin adapter not registered")
	}

	mainnetPorts := adapter.ContainerPorts(chainsv1alpha2.ChainInstanceSpec{
		Chain:   chainsv1alpha2.ChainBitcoin,
		Network: chainsv1alpha2.NetworkMainnet,
	})
	testnetPorts := adapter.ContainerPorts(chainsv1alpha2.ChainInstanceSpec{
		Chain:   chainsv1alpha2.ChainBitcoin,
		Network: chainsv1alpha2.NetworkTestnet,
	})

	findPort := func(_ []interface{}, _ string) int32 {
		// Not needed, using direct approach below
		return 0
	}
	_ = findPort

	// Mainnet should use 8332/8333, testnet should use 18332/18333
	mainRPC := mainnetPorts[0].ContainerPort
	testRPC := testnetPorts[0].ContainerPort
	if mainRPC == testRPC {
		t.Error("mainnet and testnet should use different RPC ports")
	}
}

// ---------------------------------------------------------------------------
// TON: NodePortProvider interface
// ---------------------------------------------------------------------------

func TestTONImplementsNodePortProvider(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainTON)
	if !ok {
		t.Fatal("TON adapter not registered")
	}

	npp, ok := adapter.(adapters.NodePortProvider)
	if !ok {
		t.Fatal("TON adapter does not implement NodePortProvider")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{
		Chain:   chainsv1alpha2.ChainTON,
		Network: chainsv1alpha2.NetworkMainnet,
	}
	ports := npp.NodePorts(spec, "ton-mainnet-0")
	if len(ports) == 0 {
		t.Error("TON NodePorts returned empty map")
	}
	for containerPort, nodePort := range ports {
		if containerPort <= 0 {
			t.Errorf("invalid container port: %d", containerPort)
		}
		// nodePort can be 0 (auto-assign) or positive
		if nodePort < 0 {
			t.Errorf("invalid node port: %d for container port %d", nodePort, containerPort)
		}
	}
}

func TestTONConfigIsJSON(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainTON)
	if !ok {
		t.Fatal("TON adapter not registered")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{
		Chain:   chainsv1alpha2.ChainTON,
		Network: chainsv1alpha2.NetworkMainnet,
	}
	filename, content, err := adapter.ConfigTemplate(spec)
	if err != nil {
		t.Fatal(err)
	}
	if filename != "global-config.json" {
		t.Errorf("expected filename 'global-config.json', got %q", filename)
	}
	// Content should be valid JSON (starts with {)
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		t.Error("TON config does not look like JSON")
	}
}

// ---------------------------------------------------------------------------
// TRON: StartupProbeProvider interface
// ---------------------------------------------------------------------------

func TestTRONImplementsStartupProbeProvider(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainTRON)
	if !ok {
		t.Fatal("TRON adapter not registered")
	}

	spp, ok := adapter.(adapters.StartupProbeProvider)
	if !ok {
		t.Fatal("TRON adapter does not implement StartupProbeProvider")
	}

	t.Run("mainnet", func(t *testing.T) {
		spec := chainsv1alpha2.ChainInstanceSpec{
			Chain:   chainsv1alpha2.ChainTRON,
			Network: chainsv1alpha2.NetworkMainnet,
		}
		probe := spp.StartupProbe(spec)
		if probe == nil {
			t.Fatal("nil startup probe for mainnet")
		}
		// Mainnet needs long startup (loadTransForLiteNode)
		if probe.FailureThreshold < 100 {
			t.Errorf("mainnet startup probe FailureThreshold too low: %d (expected >= 100)", probe.FailureThreshold)
		}
	})

	t.Run("testnet", func(t *testing.T) {
		spec := chainsv1alpha2.ChainInstanceSpec{
			Chain:   chainsv1alpha2.ChainTRON,
			Network: chainsv1alpha2.NetworkTestnet,
		}
		probe := spp.StartupProbe(spec)
		if probe == nil {
			t.Fatal("nil startup probe for testnet")
		}
		// Testnet should have shorter startup
		if probe.FailureThreshold > 100 {
			t.Errorf("testnet startup probe FailureThreshold too high: %d (expected <= 100)", probe.FailureThreshold)
		}
	})
}

func TestTRONImplementsContainerCommandProvider(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainTRON)
	if !ok {
		t.Fatal("TRON adapter not registered")
	}

	ccp, ok := adapter.(adapters.ContainerCommandProvider)
	if !ok {
		t.Fatal("TRON adapter does not implement ContainerCommandProvider")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chainsv1alpha2.ChainTRON}
	cmd := ccp.ContainerCommand(spec)
	if len(cmd) == 0 {
		t.Fatal("TRON ContainerCommand returned empty slice")
	}
	// Should use java (bypasses shell script)
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "java") {
		t.Error("TRON ContainerCommand should invoke java directly")
	}
}

// ---------------------------------------------------------------------------
// Cosmos: ContainerCommandProvider interface
// ---------------------------------------------------------------------------

func TestCosmosImplementsContainerCommandProvider(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainCosmos)
	if !ok {
		t.Fatal("Cosmos adapter not registered")
	}

	ccp, ok := adapter.(adapters.ContainerCommandProvider)
	if !ok {
		t.Fatal("Cosmos adapter does not implement ContainerCommandProvider")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chainsv1alpha2.ChainCosmos}
	cmd := ccp.ContainerCommand(spec)
	if len(cmd) == 0 {
		t.Fatal("Cosmos ContainerCommand returned empty slice")
	}
	// Should contain gaiad
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "gaiad") {
		t.Error("Cosmos ContainerCommand should reference gaiad")
	}
}

func TestCosmosImplementsStartupProbeProvider(t *testing.T) {
	adapter, ok := adapters.Get(chainsv1alpha2.ChainCosmos)
	if !ok {
		t.Fatal("Cosmos adapter not registered")
	}

	spp, ok := adapter.(adapters.StartupProbeProvider)
	if !ok {
		t.Fatal("Cosmos adapter does not implement StartupProbeProvider")
	}

	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chainsv1alpha2.ChainCosmos}
	probe := spp.StartupProbe(spec)
	if probe == nil {
		t.Fatal("nil startup probe")
	}
	if probe.FailureThreshold <= 0 {
		t.Error("startup probe FailureThreshold should be positive")
	}
}

// ---------------------------------------------------------------------------
// DefaultNodeSelector and DefaultLivenessProbe helpers
// ---------------------------------------------------------------------------

func TestDefaultNodeSelectorKnownGroups(t *testing.T) {
	tests := []struct {
		group   chainsv1alpha2.NodeGroup
		wantKey string
		wantVal string
	}{
		{chainsv1alpha2.NodeGroupStorage, "workload-type", "storage"},
		{chainsv1alpha2.NodeGroupBlockchain, "node-role.kubernetes.io/blockchain", "true"},
	}
	for _, tc := range tests {
		t.Run(string(tc.group), func(t *testing.T) {
			sel := adapters.DefaultNodeSelector(tc.group)
			if sel == nil {
				t.Fatal("nil selector")
			}
			val, exists := sel[tc.wantKey]
			if !exists {
				t.Errorf("missing key %q in selector %v", tc.wantKey, sel)
			}
			if val != tc.wantVal {
				t.Errorf("key %q = %q, want %q", tc.wantKey, val, tc.wantVal)
			}
		})
	}
}

func TestDefaultNodeSelectorFallback(t *testing.T) {
	// Generic groups (light/medium/heavy) return nil so pods schedule on any node.
	for _, g := range []chainsv1alpha2.NodeGroup{
		chainsv1alpha2.NodeGroupLight,
		chainsv1alpha2.NodeGroupMedium,
		chainsv1alpha2.NodeGroupHeavy,
	} {
		sel := adapters.DefaultNodeSelector(g)
		if sel != nil {
			t.Errorf("group %q: expected nil selector, got %v", g, sel)
		}
	}
}

func TestDefaultLivenessProbe(t *testing.T) {
	probe := adapters.DefaultLivenessProbe(8545)
	if probe == nil {
		t.Fatal("nil probe")
	}
	if probe.ProbeHandler.TCPSocket == nil {
		t.Fatal("expected TCP socket probe")
	}
	if probe.ProbeHandler.TCPSocket.Port.IntValue() != 8545 {
		t.Errorf("expected port 8545, got %d", probe.ProbeHandler.TCPSocket.Port.IntValue())
	}
	if probe.InitialDelaySeconds <= 0 {
		t.Error("InitialDelaySeconds should be positive")
	}
	if probe.PeriodSeconds <= 0 {
		t.Error("PeriodSeconds should be positive")
	}
}

// ---------------------------------------------------------------------------
// Optional interface checks across all adapters
// ---------------------------------------------------------------------------

func TestStartupProbeProvidersReturnValidProbe(t *testing.T) {
	for _, chain := range allChains {
		adapter, ok := adapters.Get(chain)
		if !ok {
			continue
		}
		spp, ok := adapter.(adapters.StartupProbeProvider)
		if !ok {
			continue
		}
		t.Run(string(chain), func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:   chain,
				Network: chainsv1alpha2.NetworkMainnet,
			}
			probe := spp.StartupProbe(spec)
			if probe == nil {
				t.Errorf("chain %s implements StartupProbeProvider but returns nil", chain)
				return
			}
			if probe.ProbeHandler.TCPSocket == nil &&
				probe.ProbeHandler.HTTPGet == nil &&
				probe.ProbeHandler.Exec == nil {
				t.Errorf("chain %s: startup probe has no handler", chain)
			}
			if probe.FailureThreshold <= 0 {
				t.Errorf("chain %s: startup probe FailureThreshold should be positive", chain)
			}
		})
	}
}

func TestContainerCommandProvidersReturnNonEmpty(t *testing.T) {
	for _, chain := range allChains {
		adapter, ok := adapters.Get(chain)
		if !ok {
			continue
		}
		ccp, ok := adapter.(adapters.ContainerCommandProvider)
		if !ok {
			continue
		}
		t.Run(string(chain), func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:   chain,
				Network: chainsv1alpha2.NetworkMainnet,
			}
			cmd := ccp.ContainerCommand(spec)
			if len(cmd) == 0 {
				t.Errorf("chain %s implements ContainerCommandProvider but returns empty command", chain)
			}
		})
	}
}

func TestContainerArgsProvidersReturnNonEmpty(t *testing.T) {
	for _, chain := range allChains {
		adapter, ok := adapters.Get(chain)
		if !ok {
			continue
		}
		argsP, ok := adapter.(adapters.ContainerArgsProvider)
		if !ok {
			continue
		}
		t.Run(string(chain), func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:   chain,
				Network: chainsv1alpha2.NetworkMainnet,
			}
			args := argsP.ContainerArgs(spec)
			if len(args) == 0 {
				t.Errorf("chain %s implements ContainerArgsProvider but returns empty args", chain)
			}
		})
	}
}

func TestNodePortProvidersReturnNonEmpty(t *testing.T) {
	for _, chain := range allChains {
		adapter, ok := adapters.Get(chain)
		if !ok {
			continue
		}
		npp, ok := adapter.(adapters.NodePortProvider)
		if !ok {
			continue
		}
		t.Run(string(chain), func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:   chain,
				Network: chainsv1alpha2.NetworkMainnet,
			}
			ports := npp.NodePorts(spec, "test-node-0")
			if len(ports) == 0 {
				t.Errorf("chain %s implements NodePortProvider but returns empty map", chain)
			}
		})
	}
}

func TestContainerEnvProvidersReturnNonEmpty(t *testing.T) {
	for _, chain := range allChains {
		adapter, ok := adapters.Get(chain)
		if !ok {
			continue
		}
		cep, ok := adapter.(adapters.ContainerEnvProvider)
		if !ok {
			continue
		}
		t.Run(string(chain), func(t *testing.T) {
			spec := chainsv1alpha2.ChainInstanceSpec{
				Chain:   chain,
				Network: chainsv1alpha2.NetworkMainnet,
			}
			envs := cep.ContainerEnv(spec)
			if len(envs) == 0 {
				t.Errorf("chain %s implements ContainerEnvProvider but returns empty env list", chain)
			}
		})
	}
}

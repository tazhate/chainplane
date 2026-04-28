package adapters_test

import (
	"strings"
	"testing"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/adapters"

	// Register all adapters via init()
	_ "github.com/tazhate/chainplane/internal/adapters"
)

// allChains lists every chain the operator supports — 100 total.
var allChains = []nodesv1alpha1.Chain{
	nodesv1alpha1.ChainEthereum,
	nodesv1alpha1.ChainEthereumArchive,
	nodesv1alpha1.ChainBitcoin,
	nodesv1alpha1.ChainSolana,
	nodesv1alpha1.ChainBSC,
	nodesv1alpha1.ChainTRON,
	nodesv1alpha1.ChainPolygon,
	nodesv1alpha1.ChainAvalanche,
	nodesv1alpha1.ChainLitecoin,
	nodesv1alpha1.ChainXRP,
	nodesv1alpha1.ChainStellar,
	nodesv1alpha1.ChainDash,
	nodesv1alpha1.ChainTON,
	nodesv1alpha1.ChainCosmos,
	nodesv1alpha1.ChainNear,
	nodesv1alpha1.ChainSui,
	nodesv1alpha1.ChainAptos,
	nodesv1alpha1.ChainCardano,
	nodesv1alpha1.ChainArbitrum,
	nodesv1alpha1.ChainOptimism,
	nodesv1alpha1.ChainBase,
	nodesv1alpha1.ChainBerachain,
	nodesv1alpha1.ChainCronos,
	nodesv1alpha1.ChainRonin,
	nodesv1alpha1.ChainCelo,
	nodesv1alpha1.ChainFantom,
	nodesv1alpha1.ChainGnosis,
	nodesv1alpha1.ChainMantle,
	nodesv1alpha1.ChainBlast,
	nodesv1alpha1.ChainMode,
	nodesv1alpha1.ChainZora,
	nodesv1alpha1.ChainTaiko,
	nodesv1alpha1.ChainZkSync,
	nodesv1alpha1.ChainLinea,
	nodesv1alpha1.ChainScroll,
	nodesv1alpha1.ChainDogecoin,
	nodesv1alpha1.ChainOsmosis,
	nodesv1alpha1.ChainSei,
	nodesv1alpha1.ChainEvmos,
	nodesv1alpha1.ChainKava,
	nodesv1alpha1.ChainPolkadot,
	nodesv1alpha1.ChainStarknet,
	nodesv1alpha1.ChainFilecoin,
	nodesv1alpha1.ChainMoonbeam,
	nodesv1alpha1.ChainMoonriver,
	nodesv1alpha1.ChainPolygonZkEVM,
	nodesv1alpha1.ChainMantaPacific,
	nodesv1alpha1.ChainMetis,
	nodesv1alpha1.ChainFraxtal,
	nodesv1alpha1.ChainLisk,
	nodesv1alpha1.ChainKroma,
	nodesv1alpha1.ChainBob,
	nodesv1alpha1.ChainBobaEth,
	nodesv1alpha1.ChainSoneium,
	nodesv1alpha1.ChainSwell,
	nodesv1alpha1.ChainSuperseed,
	nodesv1alpha1.ChainInk,
	nodesv1alpha1.ChainMorph,
	nodesv1alpha1.ChainAbstract,
	nodesv1alpha1.ChainMegaETH,
	nodesv1alpha1.ChainZeroNetwork,
	nodesv1alpha1.ChainZircuit,
	nodesv1alpha1.ChainImmutableZkEVM,
	nodesv1alpha1.ChainWorldchain,
	nodesv1alpha1.ChainUnichain,
	nodesv1alpha1.ChainLens,
	nodesv1alpha1.ChainPlume,
	nodesv1alpha1.ChainHemi,
	nodesv1alpha1.ChainAxelar,
	nodesv1alpha1.ChainDymension,
	nodesv1alpha1.ChainAurora,
	nodesv1alpha1.ChainHarmony,
	nodesv1alpha1.ChainRootstock,
	nodesv1alpha1.ChainTelos,
	nodesv1alpha1.ChainKlaytn,
	nodesv1alpha1.ChainBitTorrent,
	nodesv1alpha1.ChainGravityAlpha,
	nodesv1alpha1.ChainMoca,
	nodesv1alpha1.ChainEverclear,
	nodesv1alpha1.ChainDoma,
	nodesv1alpha1.ChainShibarium,
	nodesv1alpha1.ChainCore,
	nodesv1alpha1.ChainHaqq,
	nodesv1alpha1.ChainHashKey,
	nodesv1alpha1.ChainEthereumClassic,
	nodesv1alpha1.ChainCronosZkEVM,
	nodesv1alpha1.ChainSonic,
	nodesv1alpha1.ChainGoat,
	nodesv1alpha1.ChainKatana,
	nodesv1alpha1.ChainMezo,
	nodesv1alpha1.ChainPlasma,
	nodesv1alpha1.ChainPlaynance,
	nodesv1alpha1.ChainOpBNB,
	nodesv1alpha1.ChainFuse,
	nodesv1alpha1.ChainThundercore,
	nodesv1alpha1.ChainWemix,
	nodesv1alpha1.ChainViction,
	nodesv1alpha1.ChainEthereumBeacon,
	nodesv1alpha1.ChainGnosisBeacon,
	nodesv1alpha1.ChainKusama,
	nodesv1alpha1.ChainHyperliquid,
	nodesv1alpha1.ChainMonad,
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
	_, ok := adapters.Get(nodesv1alpha1.Chain("nonexistent"))
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
var noConfigChains = map[nodesv1alpha1.Chain]bool{
	nodesv1alpha1.ChainLens:         true, // ZK Stack external node
	nodesv1alpha1.ChainAbstract:     true, // ZK Stack external node
	nodesv1alpha1.ChainZeroNetwork:  true, // ZK Stack external node
	nodesv1alpha1.ChainCronosZkEVM:  true, // ZK Stack external node
	nodesv1alpha1.ChainEverclear:    true, // Arbitrum Orbit (AnyTrust)
	nodesv1alpha1.ChainPlaynance:    true, // Arbitrum Orbit L3
	nodesv1alpha1.ChainGravityAlpha: true, // Arbitrum Nitro + Celestia DA
	nodesv1alpha1.ChainPlume:        true, // Arbitrum Nitro
}

func TestAllAdaptersConfigTemplate(t *testing.T) {
	for _, network := range []nodesv1alpha1.Network{
		nodesv1alpha1.NetworkMainnet,
		nodesv1alpha1.NetworkTestnet,
	} {
		for _, chain := range allChains {
			chain := chain
			t.Run(string(chain)+"/"+string(network), func(t *testing.T) {
				adapter, ok := adapters.Get(chain)
				if !ok {
					t.Fatalf("adapter not registered for chain: %s", chain)
				}
				spec := nodesv1alpha1.BlockchainNodeSpec{
					Chain:    chain,
					Network:  network,
					NodeType: nodesv1alpha1.NodeTypeRPC,
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain: chain,
				RPC:   nodesv1alpha1.RPCSpec{Enabled: true, Port: 8545},
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain: chain,
				RPC:   nodesv1alpha1.RPCSpec{Enabled: true, Port: 8545},
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
	groups := []nodesv1alpha1.NodeGroup{
		nodesv1alpha1.NodeGroupLight,
		nodesv1alpha1.NodeGroupMedium,
		nodesv1alpha1.NodeGroupHeavy,
		nodesv1alpha1.NodeGroupArchive,
		nodesv1alpha1.NodeGroupStorage,
		nodesv1alpha1.NodeGroupBlockchain,
	}

	for _, chain := range allChains {
		chain := chain
		t.Run(string(chain), func(t *testing.T) {
			adapter, ok := adapters.Get(chain)
			if !ok {
				t.Fatalf("adapter not registered for chain: %s", chain)
			}
			for _, g := range groups {
				sel := adapter.NodeSelector(g)
				if sel == nil {
					t.Errorf("chain %s, group %s: nil node selector", chain, g)
				}
				if len(sel) == 0 {
					t.Errorf("chain %s, group %s: empty node selector", chain, g)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Ethereum: test all 4 clients return distinct images
// ---------------------------------------------------------------------------

func TestEthereumClientImages(t *testing.T) {
	adapter, ok := adapters.Get(nodesv1alpha1.ChainEthereum)
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
	eth, ok1 := adapters.Get(nodesv1alpha1.ChainEthereum)
	archive, ok2 := adapters.Get(nodesv1alpha1.ChainEthereumArchive)
	if !ok1 || !ok2 {
		t.Fatal("ethereum or ethereum-archive adapter not registered")
	}
	// Both should return the same default image (same adapter type)
	if eth.DefaultImage("") != archive.DefaultImage("") {
		t.Error("ethereum and ethereum-archive should share the same adapter and default image")
	}
}

func TestEthereumConfigTemplateClients(t *testing.T) {
	adapter, ok := adapters.Get(nodesv1alpha1.ChainEthereum)
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:    nodesv1alpha1.ChainEthereum,
				Network:  nodesv1alpha1.NetworkMainnet,
				NodeType: nodesv1alpha1.NodeTypeRPC,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainEthereum)
	if !ok {
		t.Fatal("ethereum adapter not registered")
	}

	for _, network := range []nodesv1alpha1.Network{
		nodesv1alpha1.NetworkMainnet,
		nodesv1alpha1.NetworkTestnet,
	} {
		t.Run(string(network), func(t *testing.T) {
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:    nodesv1alpha1.ChainEthereum,
				Network:  network,
				NodeType: nodesv1alpha1.NodeTypeRPC,
				Client:   "reth",
			}
			_, content, err := adapter.ConfigTemplate(spec)
			if err != nil {
				t.Fatalf("ConfigTemplate error: %v", err)
			}
			if network == nodesv1alpha1.NetworkTestnet {
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainBitcoin)
	if !ok {
		t.Fatal("bitcoin adapter not registered")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{
		Chain:   nodesv1alpha1.ChainBitcoin,
		Network: nodesv1alpha1.NetworkMainnet,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainBitcoin)
	if !ok {
		t.Fatal("bitcoin adapter not registered")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{
		Chain:   nodesv1alpha1.ChainBitcoin,
		Network: nodesv1alpha1.NetworkTestnet,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainBitcoin)
	if !ok {
		t.Fatal("bitcoin adapter not registered")
	}

	mainnetPorts := adapter.ContainerPorts(nodesv1alpha1.BlockchainNodeSpec{
		Chain:   nodesv1alpha1.ChainBitcoin,
		Network: nodesv1alpha1.NetworkMainnet,
	})
	testnetPorts := adapter.ContainerPorts(nodesv1alpha1.BlockchainNodeSpec{
		Chain:   nodesv1alpha1.ChainBitcoin,
		Network: nodesv1alpha1.NetworkTestnet,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainTON)
	if !ok {
		t.Fatal("TON adapter not registered")
	}

	npp, ok := adapter.(adapters.NodePortProvider)
	if !ok {
		t.Fatal("TON adapter does not implement NodePortProvider")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{
		Chain:   nodesv1alpha1.ChainTON,
		Network: nodesv1alpha1.NetworkMainnet,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainTON)
	if !ok {
		t.Fatal("TON adapter not registered")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{
		Chain:   nodesv1alpha1.ChainTON,
		Network: nodesv1alpha1.NetworkMainnet,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainTRON)
	if !ok {
		t.Fatal("TRON adapter not registered")
	}

	spp, ok := adapter.(adapters.StartupProbeProvider)
	if !ok {
		t.Fatal("TRON adapter does not implement StartupProbeProvider")
	}

	t.Run("mainnet", func(t *testing.T) {
		spec := nodesv1alpha1.BlockchainNodeSpec{
			Chain:   nodesv1alpha1.ChainTRON,
			Network: nodesv1alpha1.NetworkMainnet,
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
		spec := nodesv1alpha1.BlockchainNodeSpec{
			Chain:   nodesv1alpha1.ChainTRON,
			Network: nodesv1alpha1.NetworkTestnet,
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainTRON)
	if !ok {
		t.Fatal("TRON adapter not registered")
	}

	ccp, ok := adapter.(adapters.ContainerCommandProvider)
	if !ok {
		t.Fatal("TRON adapter does not implement ContainerCommandProvider")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{Chain: nodesv1alpha1.ChainTRON}
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainCosmos)
	if !ok {
		t.Fatal("Cosmos adapter not registered")
	}

	ccp, ok := adapter.(adapters.ContainerCommandProvider)
	if !ok {
		t.Fatal("Cosmos adapter does not implement ContainerCommandProvider")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{Chain: nodesv1alpha1.ChainCosmos}
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
	adapter, ok := adapters.Get(nodesv1alpha1.ChainCosmos)
	if !ok {
		t.Fatal("Cosmos adapter not registered")
	}

	spp, ok := adapter.(adapters.StartupProbeProvider)
	if !ok {
		t.Fatal("Cosmos adapter does not implement StartupProbeProvider")
	}

	spec := nodesv1alpha1.BlockchainNodeSpec{Chain: nodesv1alpha1.ChainCosmos}
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
		group   nodesv1alpha1.NodeGroup
		wantKey string
		wantVal string
	}{
		{nodesv1alpha1.NodeGroupStorage, "workload-type", "storage"},
		{nodesv1alpha1.NodeGroupBlockchain, "node-role.kubernetes.io/blockchain", "true"},
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
	sel := adapters.DefaultNodeSelector(nodesv1alpha1.NodeGroupHeavy)
	if sel == nil {
		t.Fatal("nil selector")
	}
	val, exists := sel["node-type"]
	if !exists {
		t.Error("fallback should use 'node-type' key")
	}
	if val != string(nodesv1alpha1.NodeGroupHeavy) {
		t.Errorf("expected value %q, got %q", nodesv1alpha1.NodeGroupHeavy, val)
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:   chain,
				Network: nodesv1alpha1.NetworkMainnet,
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:   chain,
				Network: nodesv1alpha1.NetworkMainnet,
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:   chain,
				Network: nodesv1alpha1.NetworkMainnet,
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:   chain,
				Network: nodesv1alpha1.NetworkMainnet,
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
			spec := nodesv1alpha1.BlockchainNodeSpec{
				Chain:   chain,
				Network: nodesv1alpha1.NetworkMainnet,
			}
			envs := cep.ContainerEnv(spec)
			if len(envs) == 0 {
				t.Errorf("chain %s implements ContainerEnvProvider but returns empty env list", chain)
			}
		})
	}
}

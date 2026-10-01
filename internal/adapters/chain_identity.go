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
	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// The tables below say which chain a node must report it is on. They are
// kept in one place rather than as a method on every adapter so that a
// wrong or missing id is one diff to review, not seventy, and so that
// adapter fixes in flight do not conflict with them.
//
// Testnet entries exist only for adapters that map NetworkTestnet to one
// specific network in their config (ethereum: sepolia, bsc: chapel,
// avalanche: fuji). Elsewhere "testnet" is not tied to a network, so there
// is no id to expect.

// mainnetEVMChainIDs is the EIP-155 chain id eth_chainId returns on mainnet,
// as listed on chainlist.org (chainid.network/chains.json).
//
// Chains with an EVM but no entry: sei and kava (the main container does not
// expose the EVM JSON-RPC port), berachain (the main container is the
// beacon-kit consensus client), hyperliquid (no EVM JSON-RPC in the node).
var mainnetEVMChainIDs = map[chainsv1alpha2.Chain]uint64{
	chainsv1alpha2.ChainAbstract:        2741,
	chainsv1alpha2.ChainArbitrum:        42161,
	chainsv1alpha2.ChainAurora:          1313161554,
	chainsv1alpha2.ChainAvalanche:       43114,
	chainsv1alpha2.ChainBase:            8453,
	chainsv1alpha2.ChainBitTorrent:      199,
	chainsv1alpha2.ChainBlast:           81457,
	chainsv1alpha2.ChainBob:             60808,
	chainsv1alpha2.ChainBobaEth:         288,
	chainsv1alpha2.ChainBSC:             56,
	chainsv1alpha2.ChainCelo:            42220,
	chainsv1alpha2.ChainCore:            1116,
	chainsv1alpha2.ChainCronos:          25,
	chainsv1alpha2.ChainCronosZkEVM:     388,
	chainsv1alpha2.ChainDoma:            97477,
	chainsv1alpha2.ChainEthereum:        1,
	chainsv1alpha2.ChainEthereumArchive: 1,
	chainsv1alpha2.ChainEthereumClassic: 61,
	chainsv1alpha2.ChainEverclear:       25327,
	chainsv1alpha2.ChainEvmos:           9001,
	chainsv1alpha2.ChainFantom:          250,
	chainsv1alpha2.ChainFraxtal:         252,
	chainsv1alpha2.ChainFuse:            122,
	chainsv1alpha2.ChainGnosis:          100,
	chainsv1alpha2.ChainGoat:            2345,
	chainsv1alpha2.ChainGravityAlpha:    1625,
	chainsv1alpha2.ChainHaqq:            11235,
	chainsv1alpha2.ChainHarmony:         1666600000,
	chainsv1alpha2.ChainHashKey:         177,
	chainsv1alpha2.ChainHemi:            43111,
	chainsv1alpha2.ChainImmutableZkEVM:  13371,
	chainsv1alpha2.ChainInk:             57073,
	chainsv1alpha2.ChainKatana:          747474,
	chainsv1alpha2.ChainKlaytn:          8217,
	chainsv1alpha2.ChainKroma:           255,
	chainsv1alpha2.ChainLens:            232,
	chainsv1alpha2.ChainLinea:           59144,
	chainsv1alpha2.ChainLisk:            1135,
	chainsv1alpha2.ChainMantaPacific:    169,
	chainsv1alpha2.ChainMantle:          5000,
	chainsv1alpha2.ChainMegaETH:         4326,
	chainsv1alpha2.ChainMetis:           1088,
	chainsv1alpha2.ChainMezo:            31612,
	chainsv1alpha2.ChainMoca:            2288,
	chainsv1alpha2.ChainMode:            34443,
	chainsv1alpha2.ChainMonad:           143,
	chainsv1alpha2.ChainMoonbeam:        1284,
	chainsv1alpha2.ChainMoonriver:       1285,
	chainsv1alpha2.ChainMorph:           2818,
	chainsv1alpha2.ChainOpBNB:           204,
	chainsv1alpha2.ChainOptimism:        10,
	chainsv1alpha2.ChainPlasma:          9745,
	chainsv1alpha2.ChainPlaynance:       1829,
	chainsv1alpha2.ChainPlume:           98866,
	chainsv1alpha2.ChainPolygon:         137,
	chainsv1alpha2.ChainPolygonZkEVM:    1101,
	chainsv1alpha2.ChainRonin:           2020,
	chainsv1alpha2.ChainRootstock:       30,
	chainsv1alpha2.ChainScroll:          534352,
	chainsv1alpha2.ChainShibarium:       109,
	chainsv1alpha2.ChainSoneium:         1868,
	chainsv1alpha2.ChainSonic:           146,
	chainsv1alpha2.ChainSuperseed:       5330,
	chainsv1alpha2.ChainSwell:           1923,
	chainsv1alpha2.ChainTaiko:           167000,
	chainsv1alpha2.ChainTelos:           40,
	chainsv1alpha2.ChainThundercore:     108,
	chainsv1alpha2.ChainUnichain:        130,
	chainsv1alpha2.ChainViction:         88,
	chainsv1alpha2.ChainWemix:           1111,
	chainsv1alpha2.ChainWorldchain:      480,
	chainsv1alpha2.ChainZeroNetwork:     543210,
	chainsv1alpha2.ChainZircuit:         48900,
	chainsv1alpha2.ChainZkSync:          324,
	chainsv1alpha2.ChainZora:            7777777,
}

// testnetEVMChainIDs is the EIP-155 chain id on the network the adapter
// renders for NetworkTestnet.
var testnetEVMChainIDs = map[chainsv1alpha2.Chain]uint64{
	chainsv1alpha2.ChainAvalanche:       43113,    // fuji
	chainsv1alpha2.ChainBSC:             97,       // chapel
	chainsv1alpha2.ChainEthereum:        11155111, // sepolia
	chainsv1alpha2.ChainEthereumArchive: 11155111, // sepolia
}

// evmRPCPaths holds the HTTP path of the Ethereum JSON-RPC for chains that do
// not serve it at "/" of their rpc port.
var evmRPCPaths = map[chainsv1alpha2.Chain]string{
	chainsv1alpha2.ChainAvalanche: "/ext/bc/C/rpc",
}

// mainnetCosmosChainIDs is the chain-id a CometBFT node reports as
// node_info.network in /status on mainnet, as listed in cosmos/chain-registry.
var mainnetCosmosChainIDs = map[chainsv1alpha2.Chain]string{
	chainsv1alpha2.ChainAxelar:    "axelar-dojo-1",
	chainsv1alpha2.ChainCosmos:    "cosmoshub-4",
	chainsv1alpha2.ChainDymension: "dymension_1100-1",
	chainsv1alpha2.ChainEvmos:     "evmos_9001-2",
	chainsv1alpha2.ChainKava:      "kava_2222-10",
	chainsv1alpha2.ChainOsmosis:   "osmosis-1",
	chainsv1alpha2.ChainSei:       "pacific-1",
}

// ExpectedEVMChainID returns the chain id eth_chainId must return on a node
// of chain running on network. ok is false when the id is not known.
func ExpectedEVMChainID(chain chainsv1alpha2.Chain, network chainsv1alpha2.Network) (id uint64, ok bool) {
	switch network {
	case chainsv1alpha2.NetworkMainnet:
		id, ok = mainnetEVMChainIDs[chain]
	case chainsv1alpha2.NetworkTestnet:
		id, ok = testnetEVMChainIDs[chain]
	}
	return id, ok
}

// EVMRPCPath returns the HTTP path of the chain's Ethereum JSON-RPC.
func EVMRPCPath(chain chainsv1alpha2.Chain) string {
	if p, ok := evmRPCPaths[chain]; ok {
		return p
	}
	return "/"
}

// ExpectedCosmosChainID returns the chain-id a CometBFT node of chain
// running on network must report as node_info.network. ok is false when it
// is not known.
func ExpectedCosmosChainID(chain chainsv1alpha2.Chain, network chainsv1alpha2.Network) (id string, ok bool) {
	if network != chainsv1alpha2.NetworkMainnet {
		return "", false
	}
	id, ok = mainnetCosmosChainIDs[chain]
	return id, ok
}

/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package adapters

import (
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// TestChainIdentityTablesRegistered catches a typo'd or removed chain key.
func TestChainIdentityTablesRegistered(t *testing.T) {
	for name, table := range map[string]map[chainsv1alpha2.Chain]uint64{
		"mainnetEVMChainIDs": mainnetEVMChainIDs,
		"testnetEVMChainIDs": testnetEVMChainIDs,
	} {
		for chain, id := range table {
			if _, ok := Get(chain); !ok {
				t.Errorf("%s: %s has no adapter", name, chain)
			}
			if id == 0 {
				t.Errorf("%s: %s has chain id 0", name, chain)
			}
		}
	}
	for chain := range evmRPCPaths {
		if _, ok := mainnetEVMChainIDs[chain]; !ok {
			t.Errorf("evmRPCPaths: %s has no chain id", chain)
		}
	}
	for chain, network := range mainnetCosmosChainIDs {
		if _, ok := Get(chain); !ok {
			t.Errorf("mainnetCosmosChainIDs: %s has no adapter", chain)
		}
		if network == "" {
			t.Errorf("mainnetCosmosChainIDs: %s has an empty chain-id", chain)
		}
	}
}

func TestExpectedEVMChainID(t *testing.T) {
	tests := []struct {
		chain   chainsv1alpha2.Chain
		network chainsv1alpha2.Network
		want    uint64
		wantOK  bool
	}{
		{chainsv1alpha2.ChainEthereum, chainsv1alpha2.NetworkMainnet, 1, true},
		{chainsv1alpha2.ChainZora, chainsv1alpha2.NetworkMainnet, 7777777, true},
		{chainsv1alpha2.ChainEthereum, chainsv1alpha2.NetworkTestnet, 11155111, true},
		{chainsv1alpha2.ChainZora, chainsv1alpha2.NetworkTestnet, 0, false},
		{chainsv1alpha2.ChainEthereum, chainsv1alpha2.NetworkDevnet, 0, false},
		{chainsv1alpha2.ChainBitcoin, chainsv1alpha2.NetworkMainnet, 0, false},
	}
	for _, tt := range tests {
		got, ok := ExpectedEVMChainID(tt.chain, tt.network)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ExpectedEVMChainID(%s, %s) = %d, %v, want %d, %v",
				tt.chain, tt.network, got, ok, tt.want, tt.wantOK)
		}
	}
	if got := EVMRPCPath(chainsv1alpha2.ChainAvalanche); got != "/ext/bc/C/rpc" {
		t.Errorf("EVMRPCPath(avalanche) = %q", got)
	}
	if got := EVMRPCPath(chainsv1alpha2.ChainEthereum); got != "/" {
		t.Errorf("EVMRPCPath(ethereum) = %q", got)
	}
	if got, ok := ExpectedCosmosChainID(chainsv1alpha2.ChainCosmos, chainsv1alpha2.NetworkMainnet); got != "cosmoshub-4" || !ok {
		t.Errorf("ExpectedCosmosChainID(cosmos) = %q, %v", got, ok)
	}
}

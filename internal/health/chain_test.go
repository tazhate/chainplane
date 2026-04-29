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
package health

import (
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

func TestChainToBlockchainType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chain chainsv1alpha2.Chain
		want  BlockchainType
	}{
		{
			name:  "Ethereum maps to BlockchainEthereum",
			chain: chainsv1alpha2.ChainEthereum,
			want:  BlockchainEthereum,
		},
		{
			name:  "EthereumArchive maps to BlockchainEthereum",
			chain: chainsv1alpha2.ChainEthereumArchive,
			want:  BlockchainEthereum,
		},
		{
			name:  "Solana maps to BlockchainSolana",
			chain: chainsv1alpha2.ChainSolana,
			want:  BlockchainSolana,
		},
		{
			name:  "Bitcoin defaults to BlockchainEthereum",
			chain: chainsv1alpha2.ChainBitcoin,
			want:  BlockchainEthereum,
		},
		{
			name:  "BSC defaults to BlockchainEthereum",
			chain: chainsv1alpha2.ChainBSC,
			want:  BlockchainEthereum,
		},
		{
			name:  "Polygon defaults to BlockchainEthereum",
			chain: chainsv1alpha2.ChainPolygon,
			want:  BlockchainEthereum,
		},
		{
			name:  "TRON defaults to BlockchainEthereum",
			chain: chainsv1alpha2.ChainTRON,
			want:  BlockchainEthereum,
		},
		{
			name:  "unknown chain defaults to BlockchainEthereum",
			chain: chainsv1alpha2.Chain("unknown-chain"),
			want:  BlockchainEthereum,
		},
		{
			name:  "empty chain defaults to BlockchainEthereum",
			chain: chainsv1alpha2.Chain(""),
			want:  BlockchainEthereum,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ChainToBlockchainType(tc.chain)
			if got != tc.want {
				t.Errorf("ChainToBlockchainType(%q) = %q, want %q", tc.chain, got, tc.want)
			}
		})
	}
}

func TestChainConfigs(t *testing.T) {
	t.Parallel()

	t.Run("Ethereum config exists", func(t *testing.T) {
		t.Parallel()
		cfg, ok := chainConfigs[BlockchainEthereum]
		if !ok {
			t.Fatal("missing config for BlockchainEthereum")
		}
		if cfg.defaultSyncLag != 30 {
			t.Errorf("defaultSyncLag = %v, want 30", cfg.defaultSyncLag)
		}
		if cfg.defaultLatencyP99 != 2.0 {
			t.Errorf("defaultLatencyP99 = %v, want 2.0", cfg.defaultLatencyP99)
		}
		if cfg.syncLagQuery == "" {
			t.Error("syncLagQuery should not be empty")
		}
	})

	t.Run("Solana config exists", func(t *testing.T) {
		t.Parallel()
		cfg, ok := chainConfigs[BlockchainSolana]
		if !ok {
			t.Fatal("missing config for BlockchainSolana")
		}
		if cfg.defaultSyncLag != 150 {
			t.Errorf("defaultSyncLag = %v, want 150", cfg.defaultSyncLag)
		}
		if cfg.defaultLatencyP99 != 0.5 {
			t.Errorf("defaultLatencyP99 = %v, want 0.5", cfg.defaultLatencyP99)
		}
	})
}

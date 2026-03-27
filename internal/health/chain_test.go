package health

import (
	"testing"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

func TestChainToBlockchainType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chain nodesv1alpha1.Chain
		want  BlockchainType
	}{
		{
			name:  "Ethereum maps to BlockchainEthereum",
			chain: nodesv1alpha1.ChainEthereum,
			want:  BlockchainEthereum,
		},
		{
			name:  "EthereumArchive maps to BlockchainEthereum",
			chain: nodesv1alpha1.ChainEthereumArchive,
			want:  BlockchainEthereum,
		},
		{
			name:  "Solana maps to BlockchainSolana",
			chain: nodesv1alpha1.ChainSolana,
			want:  BlockchainSolana,
		},
		{
			name:  "Bitcoin defaults to BlockchainEthereum",
			chain: nodesv1alpha1.ChainBitcoin,
			want:  BlockchainEthereum,
		},
		{
			name:  "BSC defaults to BlockchainEthereum",
			chain: nodesv1alpha1.ChainBSC,
			want:  BlockchainEthereum,
		},
		{
			name:  "Polygon defaults to BlockchainEthereum",
			chain: nodesv1alpha1.ChainPolygon,
			want:  BlockchainEthereum,
		},
		{
			name:  "TRON defaults to BlockchainEthereum",
			chain: nodesv1alpha1.ChainTRON,
			want:  BlockchainEthereum,
		},
		{
			name:  "unknown chain defaults to BlockchainEthereum",
			chain: nodesv1alpha1.Chain("unknown-chain"),
			want:  BlockchainEthereum,
		},
		{
			name:  "empty chain defaults to BlockchainEthereum",
			chain: nodesv1alpha1.Chain(""),
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

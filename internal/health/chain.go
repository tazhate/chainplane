package health

import (
	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// BlockchainType represents a supported blockchain network used for
// selecting the correct metric queries and threshold values.
type BlockchainType string

const (
	// BlockchainEthereum covers all EVM-compatible chains.
	BlockchainEthereum BlockchainType = "ethereum"
	// BlockchainSolana covers Solana and compatible forks.
	BlockchainSolana BlockchainType = "solana"
)

// chainMetricConfig holds chain-specific metric names and default thresholds.
type chainMetricConfig struct {
	syncLagQuery      string // PromQL metric name for sync lag (expects %s for pod)
	defaultSyncLag    float64
	defaultLatencyP99 float64
}

// chainConfigs maps each blockchain type to its metric configuration.
// Adding a new chain requires only a new entry here.
var chainConfigs = map[BlockchainType]chainMetricConfig{
	BlockchainEthereum: {
		syncLagQuery:      `eth_sync_lag{node="%s"}`,
		defaultSyncLag:    30,
		defaultLatencyP99: 2.0,
	},
	BlockchainSolana: {
		syncLagQuery:      `sol_slot_lag{node="%s"}`,
		defaultSyncLag:    150,
		defaultLatencyP99: 0.5,
	},
}

// chainMapping maps CRD Chain values to the health package BlockchainType.
// Chains not listed here default to BlockchainEthereum.
var chainMapping = map[nodesv1alpha1.Chain]BlockchainType{
	nodesv1alpha1.ChainEthereum:        BlockchainEthereum,
	nodesv1alpha1.ChainEthereumArchive: BlockchainEthereum,
	nodesv1alpha1.ChainSolana:          BlockchainSolana,
}

// ChainToBlockchainType converts a CRD Chain enum to a health BlockchainType.
// Unknown chains default to BlockchainEthereum.
func ChainToBlockchainType(chain nodesv1alpha1.Chain) BlockchainType {
	if bt, ok := chainMapping[chain]; ok {
		return bt
	}
	return BlockchainEthereum
}

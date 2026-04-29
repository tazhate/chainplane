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
	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
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
var chainMapping = map[chainsv1alpha2.Chain]BlockchainType{
	chainsv1alpha2.ChainEthereum:        BlockchainEthereum,
	chainsv1alpha2.ChainEthereumArchive: BlockchainEthereum,
	chainsv1alpha2.ChainSolana:          BlockchainSolana,
}

// ChainToBlockchainType converts a CRD Chain enum to a health BlockchainType.
// Unknown chains default to BlockchainEthereum.
func ChainToBlockchainType(chain chainsv1alpha2.Chain) BlockchainType {
	if bt, ok := chainMapping[chain]; ok {
		return bt
	}
	return BlockchainEthereum
}

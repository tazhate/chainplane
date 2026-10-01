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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// gravityAlphaChainInfo is the Orbit chain info of Gravity Alpha Mainnet
// (chain id 1625, parent chain Ethereum), as published by Conduit at
// https://api.conduit.xyz/file/v1/arbitrum/chaininfo/gravity-mainnet-0.
// Since 2025-11-18 the chain posts batches to an AnyTrust DAC instead of
// Celestia and runs on stock offchainlabs/nitro-node.
const gravityAlphaChainInfo = `[{"chain-id":1625,"parent-chain-id":1,"chain-name":"conduit-orbit-deployer","chain-config":{"chainId":1625,"homesteadBlock":0,"daoForkBlock":null,"daoForkSupport":true,"eip150Block":0,"eip150Hash":"0x0000000000000000000000000000000000000000000000000000000000000000","eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"muirGlacierBlock":0,"berlinBlock":0,"londonBlock":0,"clique":{"period":0,"epoch":0},"arbitrum":{"EnableArbOS":true,"AllowDebugPrecompiles":false,"DataAvailabilityCommittee":true,"InitialArbOSVersion":11,"InitialChainOwner":"0xd65776c5F9fA552cB5C9556B3e86bF6c376b233b","GenesisBlockNum":0}},"rollup":{"bridge":"0x7983403dDA368AA7d67145a9b81c5c517F364c42","inbox":"0x7AD2a94BefF3294a31894cFb5ba4206957a53c19","sequencer-inbox":"0x8D99372612e8cFE7163B1a453831Bc40eAeb3cF3","rollup":"0x2807B1d5d94ca823ca7d8642A5F5DDac120ce48f","validator-wallet-creator":"0x9CAd81628aB7D8e239F1A5B497313341578c5F71","deployed-at":19898364,"stake-token":"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"}}]`

// gravityAlpha is Gravity Alpha Mainnet. The feed, forwarding target and DAS
// REST aggregator are the ones from docs.gravity.xyz (legacy Alpha Mainnet
// L2 node guide).
var gravityAlpha = nitroChain{
	ChainID:          1625,
	ChainInfoJSON:    gravityAlphaChainInfo,
	ForwardingTarget: "https://rpc.gravity.xyz",
	FeedURL:          "wss://relay-gravity-mainnet-0.t.conduit.xyz",
	DASRestURL:       "https://das-gravity-mainnet-0.t.conduit.xyz",
	ParentChainURL:   "http://ethereum:8545",
	BlobsFromBeacon:  true,
	HTTPPort:         8547,
	WSPort:           8548,
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type gravityAlphaAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainGravityAlpha, &gravityAlphaAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *gravityAlphaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainGravityAlpha, client)
}

func (a *gravityAlphaAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := nitroConfig(gravityAlpha)
	return nitroConfigFile, cfg, err
}

func (a *gravityAlphaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *gravityAlphaAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return nitroPorts(gravityAlpha)
}

func (a *gravityAlphaAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return nitroArgs(gravityAlpha)
}

// ContainerEnv injects the Ethereum execution and beacon endpoints; override
// L1_RPC_URL and L1_BEACON_URL via extraEnv.
func (a *gravityAlphaAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return nitroEnv(gravityAlpha)
}

func (a *gravityAlphaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *gravityAlphaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		// Releases are tagged v<version>-<short commit>; plain v<version>
		// tags are no longer published.
		TagPattern: `^(?P<version>v\d+\.\d+\.\d+)-[0-9a-f]{7}$`,
	}
}

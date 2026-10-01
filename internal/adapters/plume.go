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

// Orbit chain info of Plume Mainnet (chain id 98866, parent Ethereum) and
// Plume Testnet (98867, parent Sepolia), as published by Conduit at
// https://api.conduit.xyz/file/v1/arbitrum/chaininfo/plume-{mainnet,testnet}-1
// and in the docs.plume.org node guide.
const (
	plumeMainnetChainInfo = `[{"chain-id":98866,"parent-chain-id":1,"chain-name":"conduit-orbit-deployer","chain-config":{"chainId":98866,"homesteadBlock":0,"daoForkBlock":null,"daoForkSupport":true,"eip150Block":0,"eip150Hash":"0x0000000000000000000000000000000000000000000000000000000000000000","eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"muirGlacierBlock":0,"berlinBlock":0,"londonBlock":0,"clique":{"period":0,"epoch":0},"arbitrum":{"EnableArbOS":true,"AllowDebugPrecompiles":false,"DataAvailabilityCommittee":true,"InitialArbOSVersion":32,"InitialChainOwner":"0x5Ec32984332eaB190cA431545664320259D755d8","GenesisBlockNum":0}},"rollup":{"bridge":"0x35381f63091926750F43b2A7401B083263aDEF83","inbox":"0x943fc691242291B74B105e8D19bd9E5DC2fcBa1D","sequencer-inbox":"0x85eC1b9138a8b9659A51e2b51bb0861901040b59","rollup":"0x4eD3F488a5a4417839BbC39712EB76D8Aaee6eE8","validator-wallet-creator":"0x0A5eC2286bB15893d5b8f320aAbc823B2186BA09","deployed-at":21887008,"stake-token":"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"}}]`
	plumeTestnetChainInfo = `[{"chain-id":98867,"parent-chain-id":11155111,"chain-name":"conduit-orbit-deployer","chain-config":{"chainId":98867,"homesteadBlock":0,"daoForkBlock":null,"daoForkSupport":true,"eip150Block":0,"eip150Hash":"0x0000000000000000000000000000000000000000000000000000000000000000","eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"muirGlacierBlock":0,"berlinBlock":0,"londonBlock":0,"clique":{"period":0,"epoch":0},"arbitrum":{"EnableArbOS":true,"AllowDebugPrecompiles":false,"DataAvailabilityCommittee":true,"InitialArbOSVersion":32,"InitialChainOwner":"0x09a24DD120676EA4034cD47BfA4432a6a87A8a42","GenesisBlockNum":0}},"rollup":{"bridge":"0xC55b89c17d7a35877FA4ea818fea2a70d5765f1c","inbox":"0xb48cdff890199f5De31514024B95F8664F8Af222","sequencer-inbox":"0xbCa991f1831bE1F1E7e5576d5F84A645e70F3E4d","rollup":"0x3B37BeD1c38c6283A56E60340FE813C0BBA031C3","validator-wallet-creator":"0x684A827456373a0C0379B1C82BA31Ee5E4F88F62","deployed-at":7889627,"stake-token":"0x7b79995e5f793A07Bc00c21412e50Ecae098E7f9"}}]`
)

// plumeChain returns Plume Mainnet, or Plume Testnet for testnet, with the
// feed, forwarding target and DAS REST aggregator from the docs.plume.org
// node guide. Plume runs stock offchainlabs/nitro-node with an AnyTrust DAC.
func plumeChain(spec chainsv1alpha2.ChainInstanceSpec) nitroChain {
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		return nitroChain{
			ChainID:          98867,
			ChainInfoJSON:    plumeTestnetChainInfo,
			ForwardingTarget: "https://testnet-rpc.plume.org",
			FeedURL:          "wss://relay-plume-testnet-1.t.conduit.xyz",
			DASRestURL:       "https://das-plume-testnet-1.t.conduit.xyz",
			ParentChainURL:   "http://ethereum:8545",
			BlobsFromBeacon:  true,
			HTTPPort:         8547,
			WSPort:           8548,
		}
	}
	return nitroChain{
		ChainID:          98866,
		ChainInfoJSON:    plumeMainnetChainInfo,
		ForwardingTarget: "https://rpc.plume.org",
		FeedURL:          "wss://relay-plume-mainnet-1.t.conduit.xyz",
		DASRestURL:       "https://das-plume-mainnet-1.t.conduit.xyz",
		ParentChainURL:   "http://ethereum:8545",
		BlobsFromBeacon:  true,
		HTTPPort:         8547,
		WSPort:           8548,
	}
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type plumeAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainPlume, &plumeAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *plumeAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainPlume, client)
}

func (a *plumeAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := nitroConfig(plumeChain(spec))
	return nitroConfigFile, cfg, err
}

func (a *plumeAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *plumeAdapter) ContainerPorts(spec chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return nitroPorts(plumeChain(spec))
}

func (a *plumeAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return nitroArgs(plumeChain(spec))
}

// ContainerEnv injects the Ethereum (Sepolia on testnet) execution and beacon
// endpoints; override L1_RPC_URL and L1_BEACON_URL via extraEnv.
func (a *plumeAdapter) ContainerEnv(spec chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return nitroEnv(plumeChain(spec))
}

func (a *plumeAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *plumeAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		// Releases are tagged v<version>-<short commit>; plain v<version>
		// tags are no longer published.
		TagPattern: `^(?P<version>v\d+\.\d+\.\d+)-[0-9a-f]{7}$`,
	}
}

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

// defaultPolygonZkEVML1URL is the in-cluster Ethereum L1 endpoint cdk-erigon
// reads the rollup state from. Override via spec.extraEnv (L1_RPC_URL).
const defaultPolygonZkEVML1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

// polygonZkEVMAdapter runs a Polygon zkEVM mainnet node on cdk-erigon, the
// Erigon-based client that replaced the now-archived zkevm-node.
type polygonZkEVMAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainPolygonZkEVM, &polygonZkEVMAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *polygonZkEVMAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainPolygonZkEVM, client)
}

func (a *polygonZkEVMAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "hermezconfig.yaml", polygonZkEVMConfig, nil
}

// ContainerArgs points cdk-erigon at the mounted config file and injects the L1
// RPC URL from the environment so it can be overridden without rewriting the
// config. Flags override config-file values in Erigon.
func (a *polygonZkEVMAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--config=/config/hermezconfig.yaml",
		"--zkevm.l1-rpc-url=$(L1_RPC_URL)",
	}
}

// ContainerEnv supplies the default L1 (Ethereum) RPC endpoint used to sync the
// zkEVM rollup state. Override by setting L1_RPC_URL in spec.extraEnv.
func (a *polygonZkEVMAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultPolygonZkEVML1URL},
	}
}

func (a *polygonZkEVMAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *polygonZkEVMAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	ports := evmPorts(30303)
	return append(ports, corev1.ContainerPort{Name: "metrics", ContainerPort: 6061, Protocol: corev1.ProtocolTCP})
}

func (a *polygonZkEVMAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "0xpolygon/cdk-erigon",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *polygonZkEVMAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2Ti"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// polygonZkEVMConfig is the cdk-erigon hermez-mainnet RPC node config. The L1
// contract addresses, sequencer RPC and datastreamer endpoint are the canonical
// Polygon zkEVM mainnet values; zkevm.l1-rpc-url is injected via ContainerArgs.
const polygonZkEVMConfig = `# Polygon zkEVM mainnet node (cdk-erigon, hermez-mainnet)
datadir: /data
chain: hermez-mainnet

http: true
http.addr: 0.0.0.0
http.port: 8545
http.api: [eth, debug, net, trace, web3, erigon, zkevm]
http.vhosts: any
http.corsdomain: any
ws: true

private.api.addr: localhost:9091
metrics: true
metrics.addr: 0.0.0.0
metrics.port: 6061

externalcl: true

zkevm.l2-chain-id: 1101
zkevm.l2-sequencer-rpc-url: https://zkevm-rpc.com
zkevm.l2-datastreamer-url: stream.zkevm-rpc.com:6900
zkevm.l1-chain-id: 1
zkevm.l1-rollup-id: 1
zkevm.address-sequencer: "0x148Ee7dAF16574cD020aFa34CC658f8F3fbd2800"
zkevm.address-zkevm: "0x519E42c24163192Dca44CD3fBDCEBF6be9130987"
zkevm.address-rollup: "0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2"
zkevm.address-ger-manager: "0x580bda1e7A0CFAe92Fa7F6c20A3794F169CE3CFb"
zkevm.l1-block-range: 20000
zkevm.l1-query-delay: 6000
zkevm.l1-first-block: 16896700
zkevm.default-gas-price: 1000000000
zkevm.max-gas-price: 0
zkevm.gas-price-factor: 0.0375
`

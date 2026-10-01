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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// defaultMetisDTLURL is the Metis data transport layer (metisdao/dtl) that
// l2geth syncs from; the DTL itself reads Ethereum. Override ROLLUP_CLIENT_HTTP
// via extraEnv.
const defaultMetisDTLURL = "http://metis-dtl:7878"

// metisBootnodes are the Andromeda mainnet bootnodes from the upstream
// replica setup (CryptoManufaktur-io/metis-docker l2geth.env).
var metisBootnodes = []string{
	"enode://61b617d2549296b0b950efcf8c1d87227d454d44362e642dde37d83092a445f2b6a6fd651d611b2893249d1e9d15c8f1d4b067142cff53280a08c9c7565c29e2@3.22.33.68:30303",
	"enode://524e335aaa2a4555fe2d54f07fc34da83d80b0407d86c606b76ce918f96a348cf91947f7c60b0eabd29f68d2669cc0ced1360636daa53d0ccc948e154d0ce21e@3.129.121.37:30303",
	"enode://c4cc213e3c5cb57b1ea6e1aa0b8a28240b03a5fa4b65ff08c53faf448182ae161d68c78fe48040da1e5d9087c6f19b60644605c4c2d78845627f09b90207e56c@3.17.198.212:30303",
	"enode://690969c716d2e4f55936a2801ab1d513598f8f2afa85be1c9f9c41e8ce1f07ee352dac2925f86fd54449d7bdd9bddc7342d33c0c46c604db19de63e21362f2cd@54.237.23.7:30303",
	"enode://9bbe2b0d7e2cd7c3de7f9674fb95059026667e9b9ce717f927b4f53132dde131fab34995a984ed47328f867e5efea4e5a27dc6a90b6b1f4106f07be215e0ece5@54.174.30.211:30303",
	"enode://2577f7c6ffafcdf311e2f79bcc56671825f00a265e5cd1f0224d8bd4b484ba1870930584f37593abfb910a76b51fa82a8b48c53d2511848680beb00c357cf3e0@52.20.251.43:30303",
	"enode://5a41bbc2a57c90b443244bc2d9c4470ee84b7af21aa27256178e53a1772950c3bf9a2b6c1a761688064aaa8456f6b4e918918cda867ce59d007ff825bbefe61d@3.126.202.64:30303",
	"enode://c0c2826d5bc7baeb2b8fe3aa8726d2572508142aa2476e2d2a667c7452a3dc78b47c1176522a9b4c3203902c3ae487ae4b656c7089334536d9c1468056799939@52.57.227.34:30303",
	"enode://86fffa408fca0afc8c9f5cf3f5831d1bc39c2a49d3690ae5219001ff7bec08cac6932dc7e94c65186f7d46daa738449f2eb361d50cde15212a8e8c84eec62605@18.193.199.155:30303",
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type metisAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMetis, &metisAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *metisAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMetis, client)
}

func (a *metisAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", metisConfig, nil
}

func (a *metisAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// metisConfig is read by l2geth (go-ethereum 1.9.10), which predates snap
// sync: SyncMode must be "full".
const metisConfig = `# Metis Andromeda l2geth configuration
[Eth]
NetworkId = 1088
SyncMode = "full"

[Node]
DataDir = "/data"
HTTPHost = "0.0.0.0"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool"]
HTTPVirtualHosts = ["*"]
HTTPCors = ["*"]
WSHost = "0.0.0.0"
WSPort = 8546
WSModules = ["eth", "net", "web3"]
WSOrigins = ["*"]

[Node.P2P]
MaxPeers = 50
`

func (a *metisAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *metisAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "metisdao/l2geth",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *metisAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{
		Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerArgs loads the rendered config and enables metrics. The image's
// geth.sh passes these to both "geth init" and the node, but not to
// "geth account import", which always writes the block signer key to
// /root/.ethereum/keystore; --keystore points the node there while the chain
// data stays in the config's DataDir on the data volume. l2geth is
// go-ethereum 1.9, which has no --metrics.addr/--metrics.port: it serves
// /debug/metrics/prometheus on the pprof listener.
func (a *metisAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--config", "/config/config.toml",
		"--keystore", "/root/.ethereum/keystore",
		"--metrics", "--pprof", "--pprofaddr", "0.0.0.0", "--pprofport", "6060",
	}
}

// ContainerEnv configures the image's geth.sh and l2geth (most l2geth flags
// read an env var) as an Andromeda mainnet replica, with the values of the
// upstream replica setup (CryptoManufaktur-io/metis-docker l2geth.env).
// geth.sh fetches the genesis from ROLLUP_STATE_DUMP_PATH and waits for the
// DTL at ROLLUP_CLIENT_HTTP. DATADIR is left unset: when it is set l2geth
// ignores both --datadir and the config and uses /root/.ethereum.
// BLOCK_SIGNER_KEY is the well-known replica key
// published by Metis; geth.sh notes it "does not have to be kept secret".
func (a *metisAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "VERBOSITY", Value: "3"},
		{Name: "NO_USB", Value: "true"},
		{Name: "USING_OVM", Value: "true"},
		{Name: "CHAIN_ID", Value: "1088"},
		{Name: "NETWORK_ID", Value: "1088"},
		{Name: "TARGET_GAS_LIMIT", Value: "1100000000"},
		{Name: "ETH1_CTC_DEPLOYMENT_HEIGHT", Value: "13626959"},
		{Name: "ETH1_SYNC_SERVICE_ENABLE", Value: "false"},
		{Name: "ROLLUP_BACKEND", Value: "l1"},
		{Name: "ROLLUP_CLIENT_HTTP", Value: defaultMetisDTLURL},
		{Name: "ROLLUP_TIMESTAMP_REFRESH", Value: "10s"},
		{Name: "ROLLUP_POLL_INTERVAL_FLAG", Value: "10s"},
		{Name: "ROLLUP_ENFORCE_FEES", Value: "true"},
		{Name: "ROLLUP_STATE_DUMP_PATH", Value: "https://metisprotocol.github.io/metis-networks/andromeda-mainnet/state-dump.latest.json"},
		{Name: "BLOCK_SIGNER_KEY", Value: "6587ae678cf4fc9a33000cdbf9f35226b71dcc6a4684a31203241f9bcfd55d27"},
		{Name: "BLOCK_SIGNER_ADDRESS", Value: "0x00000398232E2064F896018496b4b44b3D62751F"},
		{Name: "RPC_ENABLE", Value: "true"},
		{Name: "RPC_ADDR", Value: "0.0.0.0"},
		{Name: "RPC_PORT", Value: "8545"},
		{Name: "RPC_API", Value: "eth,net,web3,mvm,rollupbridge"},
		{Name: "RPC_CORS_DOMAIN", Value: "*"},
		{Name: "RPC_VHOSTS", Value: "*"},
		{Name: "WS", Value: "true"},
		{Name: "WS_ADDR", Value: "0.0.0.0"},
		{Name: "WS_PORT", Value: "8546"},
		{Name: "WS_API", Value: "eth,net,web3,mvm,rollupbridge"},
		{Name: "WS_ORIGINS", Value: "*"},
		{Name: "SEQSET_VALID_HEIGHT", Value: "15214531"},
		{Name: "DESEQBLOCK", Value: "16500000"},
		{Name: "SEQSET_CONTRACT", Value: "0x0fe382b74C3894B65c10E5C12ae60Bbd8FAf5b48"},
		{Name: "SEQ_BRIDGE_URL", Value: "https://andromeda.metis.io"},
		{Name: "BOOTNODES", Value: strings.Join(metisBootnodes, ",")},
	}
}

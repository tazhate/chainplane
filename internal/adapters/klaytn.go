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

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type klaytnAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainKlaytn, &klaytnAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8551},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *klaytnAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainKlaytn, client)
}

func (a *klaytnAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "kaia.toml", klaytnConfig, nil
}

func (a *klaytnAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *klaytnAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8551, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8552, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 32323, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 32323, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *klaytnAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--metrics",
		"--metrics.addr=0.0.0.0",
		"--metrics.port=6060",
	}
}

func (a *klaytnAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *klaytnAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "klaytn/klaytn",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const klaytnConfig = `# Kaia (Klaytn) mainnet EN (Endpoint Node) configuration
[node]
datadir = "/data"

[rpc]
http.addr = "0.0.0.0"
http.port = 8551
http.api = ["eth", "net", "web3", "kaia", "debug"]
http.vhosts = ["*"]
http.corsdomain = ["*"]
ws.addr = "0.0.0.0"
ws.port = 8552
ws.api = ["eth", "net", "web3", "kaia"]
ws.origins = ["*"]

[p2p]
port = 32323
maxpeers = 25
`

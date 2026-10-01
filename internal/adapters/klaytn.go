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

// Klaytn rebranded to Kaia in 2024; the chain keeps the "klaytn" API name.
// The node is ken from kaiachain/kaia (klaytn/klaytn is no longer published).
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
	return "kaia.yaml", klaytnConfig, nil
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

// ContainerCommand sets the entrypoint: the kaiachain/kaia image has none
// (its Cmd is /bin/bash).
func (a *klaytnAdapter) ContainerCommand(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"ken"}
}

// ContainerArgs loads the flag file and enables the Prometheus exporter. ken
// has no --metrics.addr/--metrics.port; the exporter listens on all
// interfaces at --prometheusport. The metrics flags are not picked up from
// the YAML file, so they stay on the command line.
func (a *klaytnAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--conf", "/config/kaia.yaml",
		"--metrics",
		"--prometheus",
		"--prometheusport=6060",
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
		Repository: "kaiachain/kaia",
		TagPattern: `^v(?P<version>\d+\.\d+\.\d+)$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const klaytnConfig = `# Kaia (formerly Klaytn) mainnet EN (Endpoint Node).
# ken --conf flag file: keys are the long flag names, grouped by prefix.
common:
  datadir: /data
http-rpc:
  enable: true
  addr: 0.0.0.0
  port: 8551
  api: eth,net,web3,kaia,debug
  vhosts: "*"
  cors-domain: "*"
ws-rpc:
  enable: true
  addr: 0.0.0.0
  port: 8552
  api: eth,net,web3,kaia
  origins: "*"
p2p:
  port: 32323
  max-connections: 25
`

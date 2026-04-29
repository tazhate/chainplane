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

type everclearAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainEverclear, &everclearAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8547},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *everclearAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainEverclear, client)
}

func (a *everclearAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "", "", nil
}

func (a *everclearAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *everclearAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8547, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8548, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30301, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30301, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6070, Protocol: corev1.ProtocolTCP},
	}
}

func (a *everclearAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--parent-chain.connection.url=http://ethereum:8545",
		"--chain.id=25327",
		"--http.addr=0.0.0.0",
		"--http.port=8547",
		"--ws.addr=0.0.0.0",
		"--ws.port=8548",
		"--metrics",
		"--metrics.addr=0.0.0.0",
		"--metrics.port=6070",
	}
}

func (a *everclearAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *everclearAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "offchainlabs/nitro-node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

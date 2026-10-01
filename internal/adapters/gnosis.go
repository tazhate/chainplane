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

// gnosisNode runs Nethermind on its built-in gnosis (or chiado) config.
func gnosisNode(spec chainsv1alpha2.ChainInstanceSpec) nethermindNode {
	n := nethermindNode{Config: "gnosis", P2PPort: 30303, MetricsPort: 6060}
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		n.Config = "chiado"
	}
	return n
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type gnosisAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainGnosis, &gnosisAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *gnosisAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainGnosis, client)
}

func (a *gnosisAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := gnosisNode(spec).configFile()
	return nethermindConfigFile, cfg, err
}

func (a *gnosisAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *gnosisAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(8545, 300, 30, 10, 5)
}

func (a *gnosisAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *gnosisAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return gnosisNode(spec).args()
}

func (a *gnosisAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *gnosisAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "nethermind/nethermind",
		TagPattern: `^\d+\.\d+\.\d+$`,
	}
}

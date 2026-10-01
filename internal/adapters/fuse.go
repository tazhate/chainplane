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

// fuseNode runs Fuse's Nethermind build (fusenet/node:nethermind-*) on its
// built-in fuse (or spark testnet) config, as upstream's quickstart.sh does.
// Fuse left OpenEthereum in 2024; the old fusenet/node:2.x images run
// OpenEthereum and no longer follow the chain.
func fuseNode(spec chainsv1alpha2.ChainInstanceSpec) nethermindNode {
	n := nethermindNode{Config: "fuse", P2PPort: 30303, MetricsPort: 6060}
	if spec.Network == chainsv1alpha2.NetworkTestnet {
		n.Config = "spark"
	}
	return n
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type fuseAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainFuse, &fuseAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *fuseAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainFuse, client)
}

func (a *fuseAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	cfg, err := fuseNode(spec).configFile()
	return nethermindConfigFile, cfg, err
}

func (a *fuseAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *fuseAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *fuseAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return fuseNode(spec).args()
}

func (a *fuseAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("300Gi"),
	}
}

func (a *fuseAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "fusenet/node",
		// Nethermind builds are tagged nethermind-v<version>; the plain
		// <version> tags are the retired OpenEthereum images.
		TagPattern: `^nethermind-(?P<version>v\d+\.\d+\.\d+)$`,
	}
}

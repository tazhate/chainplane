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

const defaultZkSyncL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type zksyncAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainZkSync, &zksyncAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 3060},
	})
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const zksyncConfig = `# zkSync Era external node configuration
# RPC is exposed on port 3060 by default
EN_HTTP_PORT=3060
EN_WS_PORT=3061
EN_HEALTHCHECK_PORT=3071
EN_STATE_CACHE_PATH=/data/state_keeper
EN_MERKLE_TREE_PATH=/data/tree
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *zksyncAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainZkSync, client)
}

func (a *zksyncAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "external-node.env", zksyncConfig, nil
}

func (a *zksyncAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// ContainerEnv injects the L1 Ethereum RPC URL required by zkSync Era external node.
func (a *zksyncAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "EN_ETH_CLIENT_URL", Value: defaultZkSyncL1URL},
	}
}

func (a *zksyncAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

func (a *zksyncAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "matterlabs/external-node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *zksyncAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 3060, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 3061, Protocol: corev1.ProtocolTCP},
		{Name: "health", ContainerPort: 3071, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 3312, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30303, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30303, Protocol: corev1.ProtocolUDP},
	}
}

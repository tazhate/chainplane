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

const defaultAbstractL1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type abstractAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainAbstract, &abstractAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 3060},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *abstractAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainAbstract, client)
}

func (a *abstractAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "", "", nil
}

func (a *abstractAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *abstractAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *abstractAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "matterlabs/external-node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *abstractAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 3060, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 3061, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *abstractAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

// ContainerEnv injects the L1 Ethereum RPC URL required by ZK Stack external node (Abstract).
func (a *abstractAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "EN_L1_ETH_CLIENT_WEB3_URL", Value: defaultAbstractL1URL},
	}
}

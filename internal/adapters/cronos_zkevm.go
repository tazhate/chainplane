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

const defaultCronosZkEVML1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type cronosZkEVMAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainCronosZkEVM, &cronosZkEVMAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 3060},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *cronosZkEVMAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainCronosZkEVM, client)
}

func (a *cronosZkEVMAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "", "", nil
}

func (a *cronosZkEVMAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *cronosZkEVMAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *cronosZkEVMAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "cronos-labs/external-node",
		TagPattern: `^mainnet-v\d+`,
		TagPrefix:  "mainnet-",
	}
}

func (a *cronosZkEVMAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 3060, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 3061, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 3312, Protocol: corev1.ProtocolTCP},
	}
}

func (a *cronosZkEVMAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--prometheus-port=3312"}
}

// ContainerEnv injects the L1 Ethereum RPC URL required by Cronos zkEVM ZK Stack external node.
func (a *cronosZkEVMAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "EN_L1_ETH_CLIENT_WEB3_URL", Value: defaultCronosZkEVML1URL},
	}
}

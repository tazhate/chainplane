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

// NOTE: aurora-relayer is deprecated. This uses standalone-rpc (nearaurora/srpc2-relayer). Full Aurora setup requires refiner + nearcore sidecars.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type auroraAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainAurora, &auroraAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *auroraAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainAurora, client)
}

func (a *auroraAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "aurora.toml", auroraConfig, nil
}

func (a *auroraAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *auroraAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	ports := evmPorts(30303)
	// srpc2-relayer exposes Prometheus metrics on port 9090
	return append(ports, corev1.ContainerPort{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP})
}

func (a *auroraAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const auroraConfig = `# Aurora (EVM on NEAR Protocol) relayer configuration
[server]
host = "0.0.0.0"
port = 8545

[near]
network = "mainnet"

[database]
path = "/data"
`

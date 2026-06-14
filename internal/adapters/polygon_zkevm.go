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

const defaultPolygonZkEVML1URL = "http://ethereum:8545"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type polygonZkEVMAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainPolygonZkEVM, &polygonZkEVMAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *polygonZkEVMAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainPolygonZkEVM, client)
}

func (a *polygonZkEVMAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", polygonZkEVMConfig, nil
}

func (a *polygonZkEVMAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const polygonZkEVMConfig = `# Polygon zkEVM (CDK Validium) node configuration
[Node]
DataDir = "/data"

[Node.StateDB]
Host = "localhost"
Port = 5432

[Node.Executor]
URI = "localhost:50071"

[RPC]
Host = "0.0.0.0"
Port = 8545
WebSockets = true
WebSocketsPort = 8546
ReadTimeout = "60s"
WriteTimeout = "60s"
MaxRequestsPerIPAndSecond = 500

[Metrics]
Host = "0.0.0.0"
Port = 9091
Enabled = true

[Synchronizer]
SyncInterval = "1s"
SyncChunkSize = 100
`

func (a *polygonZkEVMAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	ports := evmPorts(30303)
	return append(ports, corev1.ContainerPort{Name: "metrics", ContainerPort: 9091, Protocol: corev1.ProtocolTCP})
}

// ContainerEnv injects the L1_RPC_URL environment variable required by the zkEVM node.
func (a *polygonZkEVMAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "L1_RPC_URL", Value: defaultPolygonZkEVML1URL},
	}
}

func (a *polygonZkEVMAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

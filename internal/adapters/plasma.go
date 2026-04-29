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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// NOTE: Plasma Next (plasma.to) is a ZKP-based payment channel protocol, not a
// traditional full-node chain. This adapter is a placeholder; correct
// implementation requires Plasma-specific client.

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// plasma-next/node:v0.1.0 is a placeholder image — no public node image exists yet.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type plasmaAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainPlasma, &plasmaAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *plasmaAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainPlasma, client)
}

func (a *plasmaAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "config.toml", plasmaConfig, nil
}

func (a *plasmaAdapter) HealthCheck(_ context.Context, _ string) (SyncStatus, error) {
	return SyncStatus{}, fmt.Errorf("plasma adapter not implemented")
}

func (a *plasmaAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *plasmaAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *plasmaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *plasmaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "plasma-next/node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const plasmaConfig = `# Plasma Next placeholder configuration
# NOTE: Plasma Next is a ZKP-based payment channel protocol.
# Replace with actual Plasma-specific configuration when available.
[Node]
DataDir = "/data"
`

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

// NOTE: Monad node access is permissioned. No public Docker image confirmed as of 2026.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type monadAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMonad, &monadAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 8545},
	})
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const monadConfig = `{
  "rpc": {
    "http": true,
    "http_addr": "0.0.0.0",
    "http_port": 8545,
    "ws": true,
    "ws_addr": "0.0.0.0",
    "ws_port": 8546
  },
  "p2p": {
    "port": 30303
  },
  "data_dir": "/data"
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *monadAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMonad, client)
}

func (a *monadAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "monad.json", monadConfig, nil
}

// HealthCheck uses the standard EVM health check (eth_syncing) as Monad is EVM-compatible.
func (a *monadAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *monadAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(30303), corev1.ContainerPort{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP})
}

func (a *monadAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"--metrics", "--metrics.addr", "0.0.0.0", "--metrics.port", "6060"}
}

func (a *monadAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

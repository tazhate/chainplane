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
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// NOTE: Hyperliquid does not publish an official Docker image. hl-visor is binary-only.
// This adapter requires a custom-built image wrapping the binary from binaries.hyperliquid.xyz/Mainnet/hl-visor
// Build reference: https://github.com/CryptoManufaktur-io/hyperliquid-docker

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type hyperliquidAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainHyperliquid, &hyperliquidAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 3001},
	})
}

// --------------------------------------------------------------------------
// Config (static)
// --------------------------------------------------------------------------

const hyperliquidConfig = `{
  "api_port": 3001,
  "p2p_port": 4001,
  "data_dir": "/data"
}`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *hyperliquidAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainHyperliquid, client)
}

func (a *hyperliquidAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "hyperliquid.json", hyperliquidConfig, nil
}

// HealthCheck performs an HTTP GET to rpcURL/info.
// A 200 OK response indicates the node is alive and serving traffic.
func (a *hyperliquidAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/info", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("hyperliquid health request: %w", err)
	}

	resp, err := rpcClient.Do(req)
	if err != nil {
		if isContextTimeout(err) {
			return syncingPseudo(), nil
		}
		return SyncStatus{}, fmt.Errorf("hyperliquid health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SyncStatus{}, fmt.Errorf("hyperliquid /info returned HTTP %d", resp.StatusCode)
	}

	return SyncStatus{
		IsSyncing: false,
		Progress:  100.0,
	}, nil
}

func (a *hyperliquidAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "api", ContainerPort: 3001, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 4001, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
	}
}

func (a *hyperliquidAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("16"),
		MemoryRequest: resource.MustParse("32Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

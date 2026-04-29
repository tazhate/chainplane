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

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type rootstockAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainRootstock, &rootstockAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 4444},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *rootstockAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainRootstock, client)
}

func (a *rootstockAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "rsk.conf", rootstockConfig, nil
}

func (a *rootstockAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *rootstockAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 4444, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 5050, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 5050, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *rootstockAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "rsksmart/rskj",
		TagPattern: `^ARROWHEAD-\d+`,
		TagPrefix:  "ARROWHEAD-",
	}
}

func (a *rootstockAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const rootstockConfig = `# Rootstock (RSK) mainnet RPC node
blockchain.config.name = "main"

database.dir = "/data"

rpc {
  providers {
    web {
      cors = "*"
      http {
        enabled = true
        bind_address = "0.0.0.0"
        port = 4444
        hosts = ["*"]
      }
      ws {
        enabled = true
        bind_address = "0.0.0.0"
        port = 4445
      }
    }
  }
  modules = [
    { name: "eth", version: "1.0", enabled: true },
    { name: "net", version: "1.0", enabled: true },
    { name: "web3", version: "1.0", enabled: true },
    { name: "rsk", version: "1.0", enabled: true }
  ]
}

peer {
  port = 5050
  discovery.enabled = true
}

metrics {
  enabled = true
  server {
    bind_address = "0.0.0.0"
    port = 6060
  }
}
`

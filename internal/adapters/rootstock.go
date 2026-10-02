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

// rootstockConfigPath is where the rendered rsk.conf is mounted.
const rootstockConfigPath = "/config/rsk.conf"

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

// ContainerPorts has no metrics port: rskj serves no Prometheus endpoint
// (its expected.conf has no metrics section), so a PodMonitor would only
// scrape a closed port.
func (a *rootstockAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 4444, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 5050, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 5050, Protocol: corev1.ProtocolUDP},
	}
}

// ContainerEnv points rskj at the mounted config. The image entrypoint runs
// java with $RSKJ_SYS_PROPS and reads no config file unless -Drsk.conf.file
// is set ("user properties from -Drsk.conf.file file 'null'"), so the node
// kept its database under /var/lib/rsk/.rsk instead of /data. The image's own
// RSKJ_SYS_PROPS also sets rpc.providers.web.http.hosts.0..2, which turns
// hosts into an object that rskj rejects in favour of a list; replacing the
// variable drops those, and the config sets the bind address and hosts.
func (a *rootstockAdapter) ContainerEnv(_ chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "RSKJ_SYS_PROPS", Value: "-Drsk.conf.file=" + rootstockConfigPath},
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
  modules {
    eth { version = "1.0", enabled = true }
    net { version = "1.0", enabled = true }
    web3 { version = "1.0", enabled = true }
    rsk { version = "1.0", enabled = true }
  }
}

peer {
  port = 5050
  discovery.enabled = true
}
`

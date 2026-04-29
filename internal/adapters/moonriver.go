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

// Moonriver uses the same binary as Moonbeam; chain is selected via CLI flags.

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type moonriverAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainMoonriver, &moonriverAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 9944},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *moonriverAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainMoonriver, client)
}

func (a *moonriverAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "moonriver.json", moonriverConfig, nil
}

func (a *moonriverAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const moonriverConfig = `{
  "name": "moonriver-node",
  "rpc": {
    "port": 9944,
    "external": true,
    "cors": "all",
    "methods": "unsafe"
  },
  "ws": {
    "port": 9945,
    "external": true
  },
  "network": {
    "port": 30333,
    "nodeKey": ""
  },
  "base": {
    "path": "/data",
    "chain": "moonriver"
  }
}`

// ContainerArgs passes --base-path /data and the config file so the node uses the PVC mount.
func (a *moonriverAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{
		"--base-path", "/data",
		"--config", "/config/moonriver.json",
		"--prometheus-external",
		"--prometheus-port", "9615",
	}
}

func (a *moonriverAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("1000Gi"),
	}
}

func (a *moonriverAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "moonbeamfoundation/moonbeam",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (a *moonriverAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9944, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9945, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 30333, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 30333, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 9615, Protocol: corev1.ProtocolTCP},
	}
}

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

type harmonyAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainHarmony, &harmonyAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 9500},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *harmonyAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainHarmony, client)
}

func (a *harmonyAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "harmony.conf", harmonyConfig, nil
}

func (a *harmonyAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *harmonyAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9500, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9800, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 9000, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 9900, Protocol: corev1.ProtocolTCP},
	}
}

func (a *harmonyAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *harmonyAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "harmonyone/harmony",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const harmonyConfig = `# Harmony ONE mainnet RPC node
Version = "2.5.13"

[General]
DataDir = "/data"
EnablePruning = false
IsArchival = false
NoStaking = true
NodeType = "explorer"
ShardID = 0

[HTTP]
AuthPort = 9501
Enabled = true
IP = "0.0.0.0"
Port = 9500
RosettaEnabled = false
RosettaPort = 9700

[WS]
AuthPort = 9801
Enabled = true
IP = "0.0.0.0"
Port = 9800

[P2P]
IP = "0.0.0.0"
KeyFile = "/data/.hmykey"
Port = 9000

[Prometheus]
Enabled = true
IP = "0.0.0.0"
Port = 9900
EnablePush = false
`

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

type gnosisBeaconAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainGnosisBeacon, &gnosisBeaconAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 5052},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *gnosisBeaconAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainGnosisBeacon, client)
}

func (a *gnosisBeaconAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "lighthouse.toml", gnosisBeaconConfig, nil
}

func (a *gnosisBeaconAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return beaconHealthCheck(ctx, rpcURL)
}

func (a *gnosisBeaconAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(5052, 300, 30, 10, 5)
}

func (a *gnosisBeaconAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return beaconPorts()
}

func (a *gnosisBeaconAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *gnosisBeaconAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "sigp/lighthouse",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config (Lighthouse TOML for Gnosis network)
// --------------------------------------------------------------------------

const gnosisBeaconConfig = `# Lighthouse beacon node config for Gnosis Chain
network = "gnosis"
datadir = "/data"
http = true
http-address = "0.0.0.0"
http-port = 5052
metrics = true
metrics-address = "0.0.0.0"
metrics-port = 5054
port = 9000
discovery-port = 9000
`

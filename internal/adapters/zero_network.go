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
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

// zeroNetworkAdapter runs the ZERO Network ZK Stack external node; env, ports, probes and the
// Postgres sidecar come from zkStackProtocolAdapter.
type zeroNetworkAdapter struct {
	zkStackProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainZeroNetwork, &zeroNetworkAdapter{
		zkStackProtocolAdapter: newZkStackProtocolAdapter(
			"https://rpc.zerion.io/v1/zero", 543210, "",
		),
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *zeroNetworkAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainZeroNetwork, client)
}

func (a *zeroNetworkAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

// Zerion shut ZERO Network down on 2026-08-12 and its RPC is gone, so there
// is no default image and nothing to track. The adapter stays registered for
// existing ChainInstances that set spec.image.

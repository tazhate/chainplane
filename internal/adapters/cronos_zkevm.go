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

// cronosZkEVMAdapter runs the Cronos zkEVM ZK Stack external node; env, ports, probes and the
// Postgres sidecar come from zkStackProtocolAdapter.
type cronosZkEVMAdapter struct {
	zkStackProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainCronosZkEVM, &cronosZkEVMAdapter{
		zkStackProtocolAdapter: newZkStackProtocolAdapter(
			"https://seed.zkevm.cronos.org", 388, "cronos-zkevm-mainnet-en-snapshot",
		),
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *cronosZkEVMAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainCronosZkEVM, client)
}

func (a *cronosZkEVMAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

func (a *cronosZkEVMAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "cronos-labs/external-node",
		// mainnet-v* tags stop at v25; releases since v29 are plain vX.Y.Z.
		TagPattern: `^v(?P<version>\d+\.\d+\.\d+)$`,
	}
}

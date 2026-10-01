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

// optimismAdapter runs OP Mainnet on op-reth with an op-node sidecar; flags,
// ports and the sidecar come from opRethProtocolAdapter.
//
// OP Mainnet requires spec.snapshot. op-reth cannot sync the pre-Bedrock
// state, and v2.5.0 dropped import-op: on an empty datadir it exits with
// "Op-mainnet has been launched without importing the pre-Bedrock state".
// The operator's snapshot restore (MINIO_ENDPOINT, default bucket
// snapshots-optimism) extracts into /data, so the archive has to carry the
// op-reth datadir as reth/. The alternative is op-reth init-state
// --without-ovm from a Bedrock state dump before the first start.
type optimismAdapter struct {
	opRethProtocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainOptimism, &optimismAdapter{
		opRethProtocolAdapter: newOpRethProtocolAdapter(
			chainsv1alpha2.ChainOptimism, "optimism", "op-mainnet", "https://mainnet-sequencer.optimism.io",
		),
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *optimismAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("1Ti"),
	}
}

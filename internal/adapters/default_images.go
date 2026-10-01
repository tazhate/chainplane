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
	"maps"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// DefaultImages returns a deep copy of the generated default image table:
// chain -> client -> image reference, where client "" is the chain default.
// Callers may mutate the result freely.
func DefaultImages() map[chainsv1alpha2.Chain]map[string]string {
	out := make(map[chainsv1alpha2.Chain]map[string]string, len(chainDefaultImages))
	for chain, clients := range chainDefaultImages {
		out[chain] = maps.Clone(clients)
	}
	return out
}

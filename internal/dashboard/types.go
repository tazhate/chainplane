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
package dashboard

import "time"

// NodeListResponse is returned by GET /api/nodes.
type NodeListResponse struct {
	Nodes     []NodeInfo `json:"nodes"`
	Summary   Summary    `json:"summary"`
	FetchedAt time.Time  `json:"fetchedAt"`
}

// NodeInfo holds the flattened view of a single ChainInstance CR.
type NodeInfo struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Chain        string `json:"chain"`
	Network      string `json:"network"`
	NodeType     string `json:"nodeType"`
	Phase        string `json:"phase"`
	BlockHeight  int64  `json:"blockHeight"`
	SyncProgress string `json:"syncProgress"`
	SyncETA      string `json:"syncETA"`
	PeersCount   int32  `json:"peersCount"`
	Ready        bool   `json:"ready"`
	Age          string `json:"age"`
	Client       string `json:"client,omitempty"`
	Replicas     int32  `json:"replicas"`
}

// Summary aggregates counts by phase across all nodes.
type Summary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Syncing  int `json:"syncing"`
	Degraded int `json:"degraded"`
	Failed   int `json:"failed"`
	Pending  int `json:"pending"`
}

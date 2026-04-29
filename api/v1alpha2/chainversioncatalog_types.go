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
package v1alpha2

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const ConditionRegistryReachable = "RegistryReachable"

// ChainVersionCatalogSpec defines the desired state of ChainVersionCatalog.
type ChainVersionCatalogSpec struct {
	// Chain identifies the blockchain whose image versions are tracked.
	Chain Chain `json:"chain"`
	// Client selects the client implementation (e.g. "reth", "geth") for chains
	// that support multiple clients. Leave empty for single-client chains.
	// +optional
	Client string `json:"client,omitempty"`
	// CheckInterval is how often to poll the registry (e.g. "6h", "30m").
	// Defaults to 6h when empty.
	// +optional
	CheckInterval string `json:"checkInterval,omitempty"`
}

// TagInfo holds information about a single registry tag.
type TagInfo struct {
	Tag         string       `json:"tag"`
	PublishedAt *metav1.Time `json:"publishedAt,omitempty"`
}

// ChainVersionCatalogStatus describes the observed state of ChainVersionCatalog.
type ChainVersionCatalogStatus struct {
	// LatestTag is the most recent semver-compatible tag found in the registry.
	LatestTag string `json:"latestTag,omitempty"`
	// LatestDigest is the image digest of LatestTag (may be empty for GHCR).
	LatestDigest string `json:"latestDigest,omitempty"`
	// PublishedAt is when LatestTag was pushed to the registry.
	PublishedAt *metav1.Time `json:"publishedAt,omitempty"`
	// CheckedAt is when the operator last polled the registry.
	CheckedAt *metav1.Time `json:"checkedAt,omitempty"`
	// RecentTags lists the newest tags found (up to 10), sorted newest-first.
	RecentTags []TagInfo `json:"recentTags,omitempty"`
	// Conditions reflect the current reconciliation state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ChainVersionCatalog is a cluster-scoped resource that tracks the latest
// available image version for a given blockchain adapter by polling the
// upstream container registry on a configurable interval.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Chain",type=string,JSONPath=".spec.chain"
// +kubebuilder:printcolumn:name="Latest",type=string,JSONPath=".status.latestTag"
// +kubebuilder:printcolumn:name="CheckedAt",type=date,JSONPath=".status.checkedAt"
type ChainVersionCatalog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainVersionCatalogSpec   `json:"spec,omitempty"`
	Status ChainVersionCatalogStatus `json:"status,omitempty"`
}

// ChainVersionCatalogList contains a list of ChainVersionCatalog.
// +kubebuilder:object:root=true
type ChainVersionCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChainVersionCatalog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChainVersionCatalog{}, &ChainVersionCatalogList{})
}

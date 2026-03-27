/*
Copyright 2026.

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

package controller

import (
	"os"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
	"github.com/tazhate/blockchain-node-operator/internal/snapshot"
)

const (
	// defaultSnapshotType is used when the user does not specify a snapshot
	// variant in the CR.
	defaultSnapshotType = "full"
)

// snapshotInitContainers returns the MinIO-based snapshot restore init
// container(s) for the node pod. Returns nil when:
//   - the MINIO_ENDPOINT env var is not set on the controller pod, or
//   - spec.snapshot.disabled is true.
func (r *BlockchainNodeReconciler) snapshotInitContainers(node *nodesv1alpha1.BlockchainNode) []corev1.Container {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		return nil
	}

	if node.Spec.Snapshot != nil && node.Spec.Snapshot.Disabled {
		return nil
	}

	cfg := snapshot.Config{
		Endpoint: endpoint,
	}
	if node.Spec.Snapshot != nil {
		cfg.Bucket = node.Spec.Snapshot.Bucket
		cfg.Key = node.Spec.Snapshot.Key
		cfg.SnapshotType = string(node.Spec.Snapshot.Type)
	}
	if cfg.SnapshotType == "" {
		cfg.SnapshotType = defaultSnapshotType
	}

	return []corev1.Container{
		snapshot.BuildInitContainer(node.Spec.Chain, cfg),
	}
}

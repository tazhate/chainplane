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
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/adapters"
)

// ensureConfigMap creates or updates the chain-specific configuration
// ConfigMap. It returns a short hex hash of the rendered content so the
// StatefulSet pod template can include it as an annotation, causing a
// rolling restart whenever the configuration changes.
func (r *BlockchainNodeReconciler) ensureConfigMap(ctx context.Context, node *nodesv1alpha1.BlockchainNode, adapter adapters.ChainAdapter) (string, error) {
	r.injectRPCSecretsIntoEnv(ctx, node)

	filename, content, err := adapter.ConfigTemplate(node.Spec)
	if err != nil {
		return "", fmt.Errorf("rendering config template for %s/%s: %w", node.Namespace, node.Name, err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name + "-config",
			Namespace: node.Namespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = map[string]string{filename: content}
		return controllerutil.SetControllerReference(node, cm, r.Scheme)
	})
	if err != nil {
		return "", fmt.Errorf("upserting ConfigMap for %s/%s: %w", node.Namespace, node.Name, err)
	}

	digest := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", digest[:4]), nil
}

// injectRPCSecretsIntoEnv loads Bitcoin-family RPC credentials from a
// Kubernetes Secret into the operator process environment so that
// ConfigTemplate can interpolate them when generating the node config file.
func (r *BlockchainNodeReconciler) injectRPCSecretsIntoEnv(ctx context.Context, node *nodesv1alpha1.BlockchainNode) {
	prefixByChain := map[nodesv1alpha1.Chain]string{
		nodesv1alpha1.ChainBitcoin:  "BTC",
		nodesv1alpha1.ChainDash:     "DASH",
		nodesv1alpha1.ChainLitecoin: "LTC",
	}

	prefix, ok := prefixByChain[node.Spec.Chain]
	if !ok {
		return
	}

	reader := client.Reader(r.Client)
	if r.APIReader != nil {
		reader = r.APIReader
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{Name: node.Name + "-rpc-credentials", Namespace: node.Namespace}
	if err := reader.Get(ctx, key, secret); err != nil {
		return
	}

	if v, exists := secret.Data["rpc-user"]; exists {
		_ = os.Setenv(prefix+"_RPC_USER", string(v))
	}
	if v, exists := secret.Data["rpc-password"]; exists {
		_ = os.Setenv(prefix+"_RPC_PASSWORD", string(v))
	}
}

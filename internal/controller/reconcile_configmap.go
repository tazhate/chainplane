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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// ensureConfigMap creates or updates the chain-specific configuration
// ConfigMap. It returns a short hex hash of the rendered content so the
// StatefulSet pod template can include it as an annotation, causing a
// rolling restart whenever the configuration changes.
//
// creds are the node's RPC credentials from ensureRPCSecret; they are only
// used by adapters implementing adapters.RPCCredentialed.
func (r *ChainInstanceReconciler) ensureConfigMap(ctx context.Context, node *chainsv1alpha2.ChainInstance, adapter adapters.ChainAdapter, creds adapters.RPCCredentials) (string, error) {
	var (
		filename, content string
		err               error
	)
	if rc, ok := adapter.(adapters.RPCCredentialed); ok {
		filename, content, err = rc.ConfigTemplateWithCredentials(node.Spec, creds)
	} else {
		filename, content, err = adapter.ConfigTemplate(node.Spec)
	}
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

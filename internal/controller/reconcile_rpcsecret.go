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
package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// generatedRPCUser is the RPC user written into operator-generated Secrets.
const generatedRPCUser = "chainplane"

// rpcSecretName returns the name of the per-node Secret holding
// Bitcoin-family RPC credentials.
func rpcSecretName(node *chainsv1alpha2.ChainInstance) string {
	return node.Name + "-rpc-credentials"
}

// secretReader prefers the uncached API reader so that reading Secrets does
// not start a cluster-wide Secret informer (RBAC only grants get/create).
func (r *ChainInstanceReconciler) secretReader() client.Reader {
	if r.APIReader != nil {
		return r.APIReader
	}
	return r.Client
}

// ensureRPCSecret returns the RPC credentials of a Bitcoin-family node. When
// the "<name>-rpc-credentials" Secret does not exist it is created with a
// random password and owned by the ChainInstance; an existing (possibly
// user-provided) Secret is used as-is and never modified. Adapters that do not
// implement adapters.RPCCredentialed get zero credentials.
func (r *ChainInstanceReconciler) ensureRPCSecret(ctx context.Context, node *chainsv1alpha2.ChainInstance, adapter adapters.ChainAdapter) (adapters.RPCCredentials, error) {
	if _, ok := adapter.(adapters.RPCCredentialed); !ok {
		return adapters.RPCCredentials{}, nil
	}

	key := client.ObjectKey{Name: rpcSecretName(node), Namespace: node.Namespace}
	secret := &corev1.Secret{}
	err := r.secretReader().Get(ctx, key, secret)
	switch {
	case apierrors.IsNotFound(err):
		if secret, err = r.createRPCSecret(ctx, node); err != nil {
			return adapters.RPCCredentials{}, err
		}
	case err != nil:
		return adapters.RPCCredentials{}, fmt.Errorf("reading Secret %s/%s: %w", key.Namespace, key.Name, err)
	}

	creds := adapters.RPCCredentials{
		User:     string(secret.Data[adapters.RPCSecretUserKey]),
		Password: string(secret.Data[adapters.RPCSecretPasswordKey]),
		Salt:     string(secret.Data[adapters.RPCSecretSaltKey]),
	}
	if err := creds.Validate(); err != nil {
		return adapters.RPCCredentials{}, fmt.Errorf("invalid RPC credentials in Secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	return creds, nil
}

// createRPCSecret generates and stores fresh RPC credentials for node. The
// rpcauth salt is stored alongside so the rendered config is stable across
// reconciles.
func (r *ChainInstanceReconciler) createRPCSecret(ctx context.Context, node *chainsv1alpha2.ChainInstance) (*corev1.Secret, error) {
	password := make([]byte, 32)
	if _, err := rand.Read(password); err != nil {
		return nil, fmt.Errorf("generating RPC password: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generating rpcauth salt: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rpcSecretName(node),
			Namespace: node.Namespace,
			Labels:    coreLabels(node),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			adapters.RPCSecretUserKey:     []byte(generatedRPCUser),
			adapters.RPCSecretPasswordKey: []byte(base64.RawURLEncoding.EncodeToString(password)),
			adapters.RPCSecretSaltKey:     []byte(hex.EncodeToString(salt)),
		},
	}
	if err := controllerutil.SetControllerReference(node, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner on Secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	if err := r.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("creating Secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
		// Created concurrently (another reconcile or the user); use that one.
		existing := &corev1.Secret{}
		if err := r.secretReader().Get(ctx, client.ObjectKeyFromObject(secret), existing); err != nil {
			return nil, fmt.Errorf("reading Secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
		return existing, nil
	}

	log.FromContext(ctx).Info("created RPC credentials Secret", "secret", secret.Name)
	return secret, nil
}

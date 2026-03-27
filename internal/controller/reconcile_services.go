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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
	"github.com/tazhate/blockchain-node-operator/internal/adapters"
)

// ensureService creates or updates the Kubernetes Service that exposes the
// blockchain node's RPC and P2P ports. When the adapter implements
// NodePortProvider the service type is NodePort; otherwise ClusterIP.
//
// Kubernetes does not allow changing a Service's type in-place, so when a
// type mismatch is detected the old Service is deleted first.
func (r *BlockchainNodeReconciler) ensureService(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	adapter adapters.ChainAdapter,
) error {
	desiredType, svcPorts := r.buildServicePorts(node, adapter)

	// Handle type mismatch by deleting the stale Service before upsert.
	if err := r.deleteOnTypeMismatch(ctx, node, desiredType); err != nil {
		return err
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = selectorLabels(node)
		svc.Spec = corev1.ServiceSpec{
			Selector: selectorLabels(node),
			Ports:    svcPorts,
			Type:     desiredType,
		}
		return controllerutil.SetControllerReference(node, svc, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("upserting Service for %s/%s: %w", node.Namespace, node.Name, err)
	}
	return nil
}

// buildServicePorts resolves the desired service type and port list from the
// adapter. If the adapter implements NodePortProvider, NodePort assignments
// are applied and the type is set to NodePort.
func (r *BlockchainNodeReconciler) buildServicePorts(
	node *nodesv1alpha1.BlockchainNode,
	adapter adapters.ChainAdapter,
) (corev1.ServiceType, []corev1.ServicePort) {
	containerPorts := adapter.ContainerPorts(node.Spec)

	svcType := corev1.ServiceTypeClusterIP
	var nodePorts map[int32]int32
	if npp, ok := adapter.(adapters.NodePortProvider); ok {
		nodePorts = npp.NodePorts(node.Spec, node.Name)
		svcType = corev1.ServiceTypeNodePort
	}

	ports := make([]corev1.ServicePort, 0, len(containerPorts))
	for _, cp := range containerPorts {
		sp := corev1.ServicePort{
			Name:       cp.Name,
			Port:       cp.ContainerPort,
			TargetPort: intstr.FromInt32(cp.ContainerPort),
			Protocol:   cp.Protocol,
		}
		if np, ok := nodePorts[cp.ContainerPort]; ok {
			sp.NodePort = np
		}
		ports = append(ports, sp)
	}

	return svcType, ports
}

// deleteOnTypeMismatch removes the existing Service if its type differs from
// the desired type. This is necessary because Kubernetes forbids in-place
// ClusterIP / NodePort transitions.
func (r *BlockchainNodeReconciler) deleteOnTypeMismatch(
	ctx context.Context,
	node *nodesv1alpha1.BlockchainNode,
	desired corev1.ServiceType,
) error {
	existing := &corev1.Service{}
	key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}
	if err := r.Get(ctx, key, existing); err != nil {
		return nil // not found is fine
	}

	if existing.Spec.Type == desired {
		return nil
	}

	if err := r.Delete(ctx, existing); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting stale %s Service %s/%s: %w", existing.Spec.Type, node.Namespace, node.Name, err)
	}
	return nil
}

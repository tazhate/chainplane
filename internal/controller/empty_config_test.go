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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

var _ = Describe("Adapters without a config file", func() {
	It("creates an empty ConfigMap and the StatefulSet for zksync", func() {
		ctx := context.Background()
		nn := types.NamespacedName{Name: "era", Namespace: createTestNamespace(ctx, "no-cfg")}

		node := newTestNode(nn.Name, nn.Namespace)
		node.Spec.Chain = chainsv1alpha2.ChainZkSync
		node.Spec.Client = ""
		node.Spec.Image = nil
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })

		_, err := reconcileOnce(ctx, nn)
		Expect(err).NotTo(HaveOccurred())

		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-config", Namespace: nn.Namespace}, cm)).To(Succeed())
		Expect(cm.Data).To(BeEmpty())

		Expect(k8sClient.Get(ctx, nn, &appsv1.StatefulSet{})).To(Succeed())
	})
})

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

var _ = Describe("Chains without a public image", func() {
	It("fails the node with ImageRequired and creates no StatefulSet", func() {
		ctx := context.Background()
		nn := types.NamespacedName{Name: "cronos", Namespace: createTestNamespace(ctx, "img-req")}

		node := newTestNode(nn.Name, nn.Namespace)
		node.Spec.Chain = chainsv1alpha2.ChainCronos
		node.Spec.Client = ""
		node.Spec.Image = nil
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })

		_, err := reconcileOnce(ctx, nn)
		Expect(err).NotTo(HaveOccurred())

		got := &chainsv1alpha2.ChainInstance{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(got.Status.Phase).To(Equal(chainsv1alpha2.NodePhaseFailed))
		cond := findCondition(got.Status.Conditions, chainsv1alpha2.ConditionDegraded)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Reason).To(Equal(ReasonImageRequired))

		err = k8sClient.Get(ctx, nn, &appsv1.StatefulSet{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "no StatefulSet expected, got err=%v", err)
	})

	It("runs the chain once spec.image is set", func() {
		ctx := context.Background()
		nn := types.NamespacedName{Name: "cronos", Namespace: createTestNamespace(ctx, "img-set")}

		node := newTestNode(nn.Name, nn.Namespace)
		node.Spec.Chain = chainsv1alpha2.ChainCronos
		node.Spec.Client = ""
		node.Spec.Image = &chainsv1alpha2.ImageSpec{Repository: "example.com/cronosd", Tag: "v1.8.0"}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })

		_, err := reconcileOnce(ctx, nn)
		Expect(err).NotTo(HaveOccurred())

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("example.com/cronosd:v1.8.0"))
	})
})

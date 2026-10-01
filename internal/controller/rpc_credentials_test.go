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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// newBitcoinTestNode returns a Bitcoin ChainInstance in namespace ns.
func newBitcoinTestNode(name, ns string) *chainsv1alpha2.ChainInstance {
	node := newTestNode(name, ns)
	node.Spec.Chain = chainsv1alpha2.ChainBitcoin
	node.Spec.Client = "bitcoind"
	node.Spec.Image = &chainsv1alpha2.ImageSpec{Repository: "lncm/bitcoind", Tag: "v28.0"}
	node.Spec.RPC = chainsv1alpha2.RPCSpec{Enabled: true, Port: 8332}
	return node
}

// createTestNamespace creates a uniquely named namespace for one test.
func createTestNamespace(ctx context.Context, prefix string) string {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()),
	}}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), ns) })
	return ns.Name
}

// rpcAuthLine returns the rpcauth= line of a rendered bitcoin.conf.
func rpcAuthLine(content string) string {
	for line := range strings.Lines(content) {
		if strings.HasPrefix(line, "rpcauth=") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

var _ = Describe("UTXO RPC credentials", func() {
	It("generates a separate Secret per node and keeps passwords out of ConfigMaps", func() {
		ctx := context.Background()
		type instance struct {
			nn     types.NamespacedName
			secret *corev1.Secret
			conf   string
		}
		instances := []*instance{
			{nn: types.NamespacedName{Name: "btc", Namespace: createTestNamespace(ctx, "rpc-a")}},
			{nn: types.NamespacedName{Name: "btc", Namespace: createTestNamespace(ctx, "rpc-b")}},
		}

		for _, in := range instances {
			node := newBitcoinTestNode(in.nn.Name, in.nn.Namespace)
			Expect(k8sClient.Create(ctx, node)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })

			_, err := reconcileOnce(ctx, in.nn)
			Expect(err).NotTo(HaveOccurred())

			in.secret = &corev1.Secret{}
			secretKey := types.NamespacedName{Name: in.nn.Name + "-rpc-credentials", Namespace: in.nn.Namespace}
			Expect(k8sClient.Get(ctx, secretKey, in.secret)).To(Succeed())
			Expect(string(in.secret.Data[adapters.RPCSecretUserKey])).To(Equal("chainplane"))
			Expect(len(in.secret.Data[adapters.RPCSecretPasswordKey])).To(BeNumerically(">=", 43))
			Expect(in.secret.Data[adapters.RPCSecretSaltKey]).To(HaveLen(32))
			Expect(metav1.IsControlledBy(in.secret, node)).To(BeTrue())

			cm := &corev1.ConfigMap{}
			cmKey := types.NamespacedName{Name: in.nn.Name + "-config", Namespace: in.nn.Namespace}
			Expect(k8sClient.Get(ctx, cmKey, cm)).To(Succeed())
			in.conf = cm.Data["bitcoin.conf"]
			Expect(in.conf).NotTo(ContainSubstring("rpcpassword"))
			Expect(in.conf).NotTo(ContainSubstring("rpcuser"))
			Expect(in.conf).NotTo(ContainSubstring(string(in.secret.Data[adapters.RPCSecretPasswordKey])))
			Expect(rpcAuthLine(in.conf)).To(HavePrefix("rpcauth=chainplane:" + string(in.secret.Data[adapters.RPCSecretSaltKey]) + "$"))

			// The exporter sidecar must reference the Secret, not carry the password.
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, in.nn, sts)).To(Succeed())
			var exporter *corev1.Container
			for i := range sts.Spec.Template.Spec.Containers {
				if sts.Spec.Template.Spec.Containers[i].Name == "metrics-exporter" {
					exporter = &sts.Spec.Template.Spec.Containers[i]
				}
			}
			Expect(exporter).NotTo(BeNil())
			for _, env := range exporter.Env {
				if env.Name == "BITCOIN_RPC_PASSWORD" {
					Expect(env.Value).To(BeEmpty())
					Expect(env.ValueFrom.SecretKeyRef.Name).To(Equal(secretKey.Name))
				}
			}
		}

		a, b := instances[0], instances[1]
		Expect(a.secret.Data[adapters.RPCSecretPasswordKey]).NotTo(Equal(b.secret.Data[adapters.RPCSecretPasswordKey]))
		Expect(rpcAuthLine(a.conf)).NotTo(Equal(rpcAuthLine(b.conf)))

		// Reconciling again must not regenerate credentials or change the config.
		_, err := reconcileOnce(ctx, a.nn)
		Expect(err).NotTo(HaveOccurred())
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: a.nn.Name + "-config", Namespace: a.nn.Namespace}, cm)).To(Succeed())
		Expect(cm.Data["bitcoin.conf"]).To(Equal(a.conf))
	})

	It("uses a user-provided Secret without modifying it", func() {
		ctx := context.Background()
		ns := createTestNamespace(ctx, "rpc-user")
		nn := types.NamespacedName{Name: "btc", Namespace: ns}

		userSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "btc-rpc-credentials", Namespace: ns},
			Data: map[string][]byte{
				adapters.RPCSecretUserKey:     []byte("chainplane"),
				adapters.RPCSecretPasswordKey: []byte("hunter2"),
			},
		}
		Expect(k8sClient.Create(ctx, userSecret)).To(Succeed())

		node := newBitcoinTestNode(nn.Name, ns)
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })

		_, err := reconcileOnce(ctx, nn)
		Expect(err).NotTo(HaveOccurred())

		got := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(userSecret), got)).To(Succeed())
		Expect(got.Data).To(Equal(userSecret.Data))
		Expect(got.OwnerReferences).To(BeEmpty())

		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "btc-config", Namespace: ns}, cm)).To(Succeed())
		// Salt derived from sha256("chainplane"+"hunter2"); see TestRPCAuthKnownVectors.
		Expect(rpcAuthLine(cm.Data["bitcoin.conf"])).To(Equal(
			"rpcauth=chainplane:9998d2f7f0cdc3b6b66bcda7dd6d5a70$738666f90557aef72e26323f593bb32a9d41f3f08049367742939aff8357401d"))
	})

	It("rejects a Secret whose user would inject config lines", func() {
		ctx := context.Background()
		ns := createTestNamespace(ctx, "rpc-bad")
		nn := types.NamespacedName{Name: "btc", Namespace: ns}

		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "btc-rpc-credentials", Namespace: ns},
			Data: map[string][]byte{
				adapters.RPCSecretUserKey:     []byte("u\nrpcallowip=0.0.0.0/0"),
				adapters.RPCSecretPasswordKey: []byte("p"),
			},
		})).To(Succeed())

		node := newBitcoinTestNode(nn.Name, ns)
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), node) })

		_, err := reconcileOnce(ctx, nn)
		Expect(err).To(MatchError(ContainSubstring("invalid RPC credentials")))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "btc-config", Namespace: ns}, &corev1.ConfigMap{})).NotTo(Succeed())
	})
})

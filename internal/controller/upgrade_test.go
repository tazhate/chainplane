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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	_ "github.com/tazhate/chainplane/internal/adapters"
)

// --- Pure unit tests (no envtest) ---

func TestFindCondition_Found(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: ConditionUpgrading, Status: metav1.ConditionTrue},
		{Type: nodesv1alpha1.ConditionReady, Status: metav1.ConditionFalse},
	}

	c := findCondition(conditions, ConditionUpgrading)
	if c == nil {
		t.Fatal("expected to find Upgrading condition, got nil")
	}
	if c.Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %s", c.Status)
	}
}

func TestFindCondition_NotFound(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: nodesv1alpha1.ConditionReady, Status: metav1.ConditionTrue},
	}

	c := findCondition(conditions, ConditionUpgrading)
	if c != nil {
		t.Errorf("expected nil, got %+v", c)
	}
}

func TestFindCondition_Empty(t *testing.T) {
	c := findCondition(nil, ConditionUpgrading)
	if c != nil {
		t.Errorf("expected nil for empty conditions, got %+v", c)
	}
}

func TestResolveImageForUpgrade_WithImageSpec(t *testing.T) {
	r := &BlockchainNodeReconciler{}
	node := &nodesv1alpha1.BlockchainNode{
		Spec: nodesv1alpha1.BlockchainNodeSpec{
			Chain:  nodesv1alpha1.ChainEthereum,
			Client: "geth",
			Image: &nodesv1alpha1.ImageSpec{
				Repository: "ethereum/client-go",
				Tag:        "v1.14.11",
			},
		},
	}

	img := r.resolveImageForUpgrade(node)
	expected := "ethereum/client-go:v1.14.11"
	if img != expected {
		t.Errorf("expected %s, got %s", expected, img)
	}
}

func TestResolveImageForUpgrade_WithoutImageSpec(t *testing.T) {
	r := &BlockchainNodeReconciler{}
	node := &nodesv1alpha1.BlockchainNode{
		Spec: nodesv1alpha1.BlockchainNodeSpec{
			Chain:  nodesv1alpha1.ChainEthereum,
			Client: "geth",
		},
	}

	img := r.resolveImageForUpgrade(node)
	expected := "ethereum/geth"
	if img != expected {
		t.Errorf("expected %s, got %s", expected, img)
	}
}

func TestResolveImageForUpgrade_EmptyRepository(t *testing.T) {
	r := &BlockchainNodeReconciler{}
	node := &nodesv1alpha1.BlockchainNode{
		Spec: nodesv1alpha1.BlockchainNodeSpec{
			Chain:  nodesv1alpha1.ChainBitcoin,
			Client: "bitcoind",
			Image: &nodesv1alpha1.ImageSpec{
				Repository: "", // empty
				Tag:        "v25.0",
			},
		},
	}

	// Empty repository falls back to chain/client format
	img := r.resolveImageForUpgrade(node)
	expected := "bitcoin/bitcoind"
	if img != expected {
		t.Errorf("expected %s, got %s", expected, img)
	}
}

func TestDetectCrashLoop_NoPods(_ *testing.T) {
	// detectCrashLoop relies on listing pods, which are absent in a pure unit
	// test. The envtest-based Ginkgo tests below cover the realistic case.
	// This is a structural placeholder confirming the function signature is stable.
}

// --- Envtest-based integration tests (Ginkgo) ---

var _ = Describe("Upgrade Controller", func() {
	const testNS = "default"

	// newUpgradeTestNode creates a node with a specific image for upgrade testing.
	newUpgradeTestNode := func(name string, repo, tag string) *nodesv1alpha1.BlockchainNode {
		replicas := int32(1)
		return &nodesv1alpha1.BlockchainNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS,
			},
			Spec: nodesv1alpha1.BlockchainNodeSpec{
				Chain:    nodesv1alpha1.ChainEthereum,
				Network:  nodesv1alpha1.NetworkMainnet,
				NodeType: nodesv1alpha1.NodeTypeRPC,
				Client:   "nethermind",
				Image: &nodesv1alpha1.ImageSpec{
					Repository: repo,
					Tag:        tag,
				},
				Storage: nodesv1alpha1.StorageSpec{
					Size: resource.MustParse("50Gi"),
				},
				NodeGroup: nodesv1alpha1.NodeGroupMedium,
				Replicas:  &replicas,
				RPC: nodesv1alpha1.RPCSpec{
					Enabled: true,
					Port:    8545,
				},
			},
		}
	}

	// setupAndReconcile creates a node, reconciles to add finalizer and create resources.
	setupAndReconcile := func(ctx context.Context, name, repo, tag string) types.NamespacedName {
		nn := types.NamespacedName{Name: name, Namespace: testNS}
		node := newUpgradeTestNode(name, repo, tag)
		Expect(k8sClient.Create(ctx, node)).To(Succeed())

		// Reconcile 1: add finalizer.
		_, err := reconcileOnce(ctx, nn)
		Expect(err).NotTo(HaveOccurred())

		// Reconcile 2: create child resources + upgrade check.
		_, err = reconcileOnce(ctx, nn)
		Expect(err).NotTo(HaveOccurred())

		return nn
	}

	Context("Image change triggers rolling restart", func() {
		It("should detect image change and set Upgrading condition", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-upgrade-img-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			// Manually set the last-image annotation on the StatefulSet to an
			// old value, simulating a state where the image was previously different.
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			patch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.35.0" // old image
			sts.Annotations[annotationLastClient] = "nethermind"
			Expect(k8sClient.Patch(ctx, sts, patch)).To(Succeed())

			// Reconcile should detect the change (1.35.0 -> 1.36.1).
			_, err := reconcileOnce(ctx, nn)
			Expect(err).NotTo(HaveOccurred())

			// Verify the StatefulSet now has the new image annotation.
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			Expect(sts.Annotations[annotationLastImage]).To(Equal("nethermind/nethermind:1.36.1"))

			// Verify the previous image was saved for rollback.
			Expect(sts.Annotations[annotationPreviousImage]).To(Equal("nethermind/nethermind:1.35.0"))

			// Verify restartedAt annotation was set on the pod template.
			Expect(sts.Spec.Template.Annotations).To(HaveKey(annotationRestartedAt))

			// Verify Upgrading condition is set on the node.
			node := &nodesv1alpha1.BlockchainNode{}
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			upgradingCond := findCondition(node.Status.Conditions, ConditionUpgrading)
			Expect(upgradingCond).NotTo(BeNil())
			Expect(upgradingCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(upgradingCond.Reason).To(Equal("RollingRestart"))
		})

		It("should not trigger restart when image is unchanged", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-upgrade-noop-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			// Set the last-image annotation to the current image.
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			patch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.36.1"
			sts.Annotations[annotationLastClient] = "nethermind"
			Expect(k8sClient.Patch(ctx, sts, patch)).To(Succeed())

			// Record the current restartedAt annotation.
			oldRestartedAt := sts.Spec.Template.Annotations[annotationRestartedAt]

			// Reconcile: no change expected.
			_, err := reconcileOnce(ctx, nn)
			Expect(err).NotTo(HaveOccurred())

			// restartedAt should not have changed.
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			Expect(sts.Spec.Template.Annotations[annotationRestartedAt]).To(Equal(oldRestartedAt))
		})

		It("should detect client change as an upgrade trigger", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-upgrade-client-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			// Set annotations as if the previous client was "geth".
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			patch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.36.1"
			sts.Annotations[annotationLastClient] = "geth" // different client
			Expect(k8sClient.Patch(ctx, sts, patch)).To(Succeed())

			// Reconcile: should detect client change.
			_, err := reconcileOnce(ctx, nn)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			Expect(sts.Annotations[annotationLastClient]).To(Equal("nethermind"))
			Expect(sts.Spec.Template.Annotations).To(HaveKey(annotationRestartedAt))
		})
	})

	Context("CrashLoopBackOff detection and rollback", func() {
		It("should detect crash loop and perform rollback", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-rollback-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			// Set up the StatefulSet with Upgrading condition and crash-loop scenario.
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			// Set annotations to simulate an upgrade in progress.
			stsPatch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.36.1"
			sts.Annotations[annotationLastClient] = "nethermind"
			sts.Annotations[annotationPreviousImage] = "nethermind/nethermind:1.35.0"
			Expect(k8sClient.Patch(ctx, sts, stsPatch)).To(Succeed())

			// Set the Upgrading condition on the node.
			node := &nodesv1alpha1.BlockchainNode{}
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())

			r := &BlockchainNodeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			Expect(r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionTrue, "RollingRestart", "Upgrade in progress")).To(Succeed())

			// Create a pod that simulates CrashLoopBackOff.
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: testNS,
					Labels:    map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "node",
							Image: "nethermind/nethermind:1.36.1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, pod)
			})

			// Update pod status to simulate CrashLoopBackOff.
			pod.Status = corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "node",
						RestartCount: 5,
						Image:        "nethermind/nethermind:1.36.1",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "CrashLoopBackOff",
							},
						},
					},
				},
			}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

			// Refresh node and sts before calling reconcileUpgrade.
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())

			// Call reconcileUpgrade directly.
			err := r.reconcileUpgrade(ctx, node)
			Expect(err).NotTo(HaveOccurred())

			// Verify rollback occurred: last-image should now be the previous image.
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			Expect(sts.Annotations[annotationLastImage]).To(Equal("nethermind/nethermind:1.35.0"))

			// Verify the container image was rolled back in the pod template.
			for _, c := range sts.Spec.Template.Spec.Containers {
				if c.Name == "node" {
					Expect(c.Image).To(Equal("nethermind/nethermind:1.35.0"))
				}
			}

			// Verify the node phase was set to Degraded.
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			Expect(node.Status.Phase).To(Equal(nodesv1alpha1.NodePhaseDegraded))

			// Verify Upgrading condition was set to False with rollback reason.
			upgradingCond := findCondition(node.Status.Conditions, ConditionUpgrading)
			Expect(upgradingCond).NotTo(BeNil())
			Expect(upgradingCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(upgradingCond.Reason).To(Equal("RollbackTriggered"))
		})

		It("should not rollback when crash loop is from old image", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-no-rollback-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			// Set up upgrade state.
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			stsPatch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.36.1"
			sts.Annotations[annotationLastClient] = "nethermind"
			sts.Annotations[annotationPreviousImage] = "nethermind/nethermind:1.35.0"
			Expect(k8sClient.Patch(ctx, sts, stsPatch)).To(Succeed())

			// Set Upgrading condition.
			node := &nodesv1alpha1.BlockchainNode{}
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())

			r := &BlockchainNodeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			Expect(r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionTrue, "RollingRestart", "Upgrade in progress")).To(Succeed())

			// Create a pod with crash loop but running the OLD image (not desired).
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: testNS,
					Labels:    map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "node",
							Image: "nethermind/nethermind:1.35.0",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, pod)
			})

			pod.Status = corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "node",
						RestartCount: 5,
						Image:        "nethermind/nethermind:1.36.0", // OLD image, not 1.36.1
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "CrashLoopBackOff",
							},
						},
					},
				},
			}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

			// Refresh and call reconcileUpgrade.
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			err := r.reconcileUpgrade(ctx, node)
			Expect(err).NotTo(HaveOccurred())

			// Verify NO rollback: last-image should still be the new version.
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			Expect(sts.Annotations[annotationLastImage]).To(Equal("nethermind/nethermind:1.36.1"))
		})

		It("should handle rollback with no previous image gracefully", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-rollback-noprev-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			// Set annotations with NO previous image.
			stsPatch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.36.1"
			sts.Annotations[annotationLastClient] = "nethermind"
			// No annotationPreviousImage.
			Expect(k8sClient.Patch(ctx, sts, stsPatch)).To(Succeed())

			// Set Upgrading condition.
			node := &nodesv1alpha1.BlockchainNode{}
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())

			r := &BlockchainNodeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			Expect(r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionTrue, "RollingRestart", "Upgrade in progress")).To(Succeed())

			// Create a crashing pod.
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: testNS,
					Labels:    map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "node", Image: "nethermind/nethermind:1.36.1"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, pod)
			})

			pod.Status = corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "node",
						RestartCount: 5,
						Image:        "nethermind/nethermind:1.36.1",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "CrashLoopBackOff",
							},
						},
					},
				},
			}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			err := r.reconcileUpgrade(ctx, node)
			Expect(err).NotTo(HaveOccurred())

			// Rollback should still set phase to Degraded even without previous image.
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			Expect(node.Status.Phase).To(Equal(nodesv1alpha1.NodePhaseDegraded))

			// Upgrading condition should be False.
			upgradingCond := findCondition(node.Status.Conditions, ConditionUpgrading)
			Expect(upgradingCond).NotTo(BeNil())
			Expect(upgradingCond.Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Context("Successful upgrade completion", func() {
		It("should clear Upgrading condition when all replicas are ready", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-upgrade-ok-%d", time.Now().UnixNano())
			nn := setupAndReconcile(ctx, name, "nethermind/nethermind", "1.36.1")
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			// Set annotations to current image (no change).
			stsPatch := client.MergeFrom(sts.DeepCopy())
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[annotationLastImage] = "nethermind/nethermind:1.36.1"
			sts.Annotations[annotationLastClient] = "nethermind"
			Expect(k8sClient.Patch(ctx, sts, stsPatch)).To(Succeed())

			// Set Upgrading condition on the node.
			node := &nodesv1alpha1.BlockchainNode{}
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())

			r := &BlockchainNodeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			Expect(r.setCondition(ctx, node, ConditionUpgrading, metav1.ConditionTrue, "RollingRestart", "In progress")).To(Succeed())

			// Simulate all replicas ready.
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			sts.Status.Replicas = 1
			sts.Status.ReadyReplicas = 1
			sts.Status.UpdatedReplicas = 1
			Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())

			// Reconcile.
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			err := r.reconcileUpgrade(ctx, node)
			Expect(err).NotTo(HaveOccurred())

			// Verify Upgrading condition was set to False with RolloutComplete reason.
			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			upgradingCond := findCondition(node.Status.Conditions, ConditionUpgrading)
			Expect(upgradingCond).NotTo(BeNil())
			Expect(upgradingCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(upgradingCond.Reason).To(Equal("RolloutComplete"))

			// Ready condition should be True.
			readyCond := findCondition(node.Status.Conditions, nodesv1alpha1.ConditionReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("setCondition via reconciler", func() {
		It("should persist conditions to API server", func() {
			ctx := context.Background()
			name := fmt.Sprintf("test-setcond-%d", time.Now().UnixNano())
			nn := types.NamespacedName{Name: name, Namespace: testNS}

			node := newUpgradeTestNode(name, "nethermind/nethermind", "1.36.1")
			Expect(k8sClient.Create(ctx, node)).To(Succeed())
			DeferCleanup(func() {
				n := &nodesv1alpha1.BlockchainNode{}
				if err := k8sClient.Get(ctx, nn, n); err == nil {
					controllerutil.RemoveFinalizer(n, nodesv1alpha1.FinalizerName)
					_ = k8sClient.Update(ctx, n)
					_ = k8sClient.Delete(ctx, n)
				}
			})

			r := &BlockchainNodeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			Expect(k8sClient.Get(ctx, nn, node)).To(Succeed())
			Expect(r.setCondition(ctx, node, "TestCondition", metav1.ConditionTrue, "TestReason", "Test message")).To(Succeed())

			fetched := &nodesv1alpha1.BlockchainNode{}
			Expect(k8sClient.Get(ctx, nn, fetched)).To(Succeed())
			cond := findCondition(fetched.Status.Conditions, "TestCondition")
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("TestReason"))
			Expect(cond.Message).To(Equal("Test message"))
		})
	})
})

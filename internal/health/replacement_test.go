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
package health

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(chainsv1alpha2.AddToScheme(s))
	return s
}

func testNode(name, ns string, chain chainsv1alpha2.Chain) *chainsv1alpha2.ChainInstance {
	return &chainsv1alpha2.ChainInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: chainsv1alpha2.ChainInstanceSpec{
			Chain: chain,
		},
	}
}

func testPod(name, ns string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "node", Image: "eth:latest"},
			},
		},
	}
}

// stubDrainer implements Drainer for testing, recording calls.
type stubDrainer struct {
	drainCalled  bool
	switchCalled bool
	validateOK   bool
	validateErr  error
	drainErr     error
	switchErr    error
}

func (d *stubDrainer) Drain(_ context.Context, _, _ string) error {
	d.drainCalled = true
	return d.drainErr
}

func (d *stubDrainer) SwitchTraffic(_ context.Context, _, _, _ string) error {
	d.switchCalled = true
	return d.switchErr
}

func (d *stubDrainer) ValidateTraffic(_ context.Context, _, _ string) (bool, error) {
	return d.validateOK, d.validateErr
}

func newTestReplacementManager(objs []client.Object, drainer Drainer) (*ReplacementManager, client.Client) {
	s := testScheme()
	fb := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...)
	k := fb.Build()

	mq := newMockQuerier()
	mq.instantValues["*"] = 0
	mq.sustainedMap["*"] = false
	valBytes, _ := json.Marshal("0")
	tsBytes, _ := json.Marshal(float64(time.Now().Unix()))
	mq.rangeValues["*"] = []MatrixSeries{{
		Values: [][2]json.RawMessage{{tsBytes, valBytes}},
	}}
	checker := NewChecker(mq, Thresholds{})

	cfg := ReplacementConfig{
		DrainTimeout:       1 * time.Second,
		SyncTimeout:        10 * time.Minute,
		PollInterval:       100 * time.Millisecond,
		GracePeriodSeconds: 0,
		CleanupPVC:         false,
	}
	rm := NewReplacementManager(k, checker, drainer, cfg)
	return rm, k
}

func TestStartReplacement(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	oldPod := testPod("eth-node-0", "default", map[string]string{"app": "eth"})
	drainer := &stubDrainer{}

	rm, k := newTestReplacementManager([]client.Object{node, oldPod}, drainer)
	ctx := context.Background()

	err := rm.StartReplacement(ctx, node)
	if err != nil {
		t.Fatalf("StartReplacement failed: %v", err)
	}

	if !drainer.drainCalled {
		t.Error("expected drain to be called")
	}

	// Check replacement pod was created.
	var repPod corev1.Pod
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node-0-replacement", Namespace: "default"}, &repPod)
	if err != nil {
		t.Fatalf("replacement pod not found: %v", err)
	}
	if repPod.Labels[LabelInstance] != "replacement" {
		t.Errorf("replacement pod missing instance label, got: %v", repPod.Labels)
	}
	if repPod.Labels[LabelReplacementFor] != "eth-node-0" {
		t.Errorf("replacement-for label = %q, want %q", repPod.Labels[LabelReplacementFor], "eth-node-0")
	}

	// Check annotations on node.
	var updated chainsv1alpha2.ChainInstance
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node", Namespace: "default"}, &updated)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updated.Annotations[AnnotationReplacementPhase] != string(ReplacementVerifying) {
		t.Errorf("phase = %q, want %q", updated.Annotations[AnnotationReplacementPhase], ReplacementVerifying)
	}
}

func TestStartReplacement_NoDrainer(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	oldPod := testPod("eth-node-0", "default", map[string]string{"app": "eth"})

	rm, _ := newTestReplacementManager([]client.Object{node, oldPod}, nil)
	ctx := context.Background()

	err := rm.StartReplacement(ctx, node)
	if err != nil {
		t.Fatalf("StartReplacement without drainer failed: %v", err)
	}
}

func TestMonitorReplacement_NotReady(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	node.Annotations = map[string]string{
		AnnotationReplacementPhase:     string(ReplacementVerifying),
		AnnotationReplacementPod:       "eth-node-0-replacement",
		AnnotationReplacementOldPod:    "eth-node-0",
		AnnotationReplacementStartedAt: time.Now().UTC().Format(time.RFC3339),
	}

	repPod := testPod("eth-node-0-replacement", "default", map[string]string{
		LabelInstance: "replacement",
	})
	repPod.Status.Phase = corev1.PodPending

	rm, _ := newTestReplacementManager([]client.Object{node, repPod}, &stubDrainer{})
	ctx := context.Background()

	ready, err := rm.MonitorReplacement(ctx, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Error("expected not ready for pending pod")
	}
}

func TestMonitorReplacement_WrongPhase(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	node.Annotations = map[string]string{
		AnnotationReplacementPhase: string(ReplacementDraining),
	}

	rm, _ := newTestReplacementManager([]client.Object{node}, &stubDrainer{})
	_, err := rm.MonitorReplacement(context.Background(), node)
	if err == nil {
		t.Fatal("expected error for wrong phase")
	}
}

func TestMonitorReplacement_MissingAnnotation(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	node.Annotations = map[string]string{
		AnnotationReplacementPhase: string(ReplacementVerifying),
		// Missing AnnotationReplacementPod.
	}

	rm, _ := newTestReplacementManager([]client.Object{node}, &stubDrainer{})
	_, err := rm.MonitorReplacement(context.Background(), node)
	if err == nil {
		t.Fatal("expected error for missing pod annotation")
	}
}

func TestCompleteReplacement(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	node.Annotations = map[string]string{
		AnnotationReplacementPhase:     string(ReplacementVerifying),
		AnnotationReplacementPod:       "eth-node-0-replacement",
		AnnotationReplacementOldPod:    "eth-node-0",
		AnnotationReplacementStartedAt: time.Now().UTC().Format(time.RFC3339),
	}

	oldPod := testPod("eth-node-0", "default", map[string]string{"app": "eth"})
	newPod := testPod("eth-node-0-replacement", "default", map[string]string{
		LabelInstance: "replacement",
	})

	drainer := &stubDrainer{validateOK: true}
	rm, k := newTestReplacementManager([]client.Object{node, oldPod, newPod}, drainer)
	ctx := context.Background()

	err := rm.CompleteReplacement(ctx, node)
	if err != nil {
		t.Fatalf("CompleteReplacement failed: %v", err)
	}

	if !drainer.switchCalled {
		t.Error("expected SwitchTraffic to be called")
	}

	// Old pod should be deleted.
	var pod corev1.Pod
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node-0", Namespace: "default"}, &pod)
	if err == nil {
		t.Error("old pod should have been deleted")
	}

	// Annotations should be cleared.
	var updated chainsv1alpha2.ChainInstance
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node", Namespace: "default"}, &updated)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if _, ok := updated.Annotations[AnnotationReplacementPhase]; ok {
		t.Error("replacement phase annotation should be cleared")
	}
}

func TestRollbackReplacement(t *testing.T) {
	t.Parallel()

	node := testNode("eth-node", "default", chainsv1alpha2.ChainEthereum)
	node.Annotations = map[string]string{
		AnnotationReplacementPhase:  string(ReplacementVerifying),
		AnnotationReplacementPod:    "eth-node-0-replacement",
		AnnotationReplacementOldPod: "eth-node-0",
	}

	oldPod := testPod("eth-node-0", "default", map[string]string{
		"app":      "eth",
		LabelReady: "false",
	})
	repPod := testPod("eth-node-0-replacement", "default", map[string]string{
		LabelInstance: "replacement",
	})

	rm, k := newTestReplacementManager([]client.Object{node, oldPod, repPod}, &stubDrainer{})
	ctx := context.Background()

	err := rm.RollbackReplacement(ctx, node)
	if err != nil {
		t.Fatalf("RollbackReplacement failed: %v", err)
	}

	// Replacement pod should be deleted.
	var pod corev1.Pod
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node-0-replacement", Namespace: "default"}, &pod)
	if err == nil {
		t.Error("replacement pod should have been deleted")
	}

	// Old pod should have ready=true label restored.
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node-0", Namespace: "default"}, &pod)
	if err != nil {
		t.Fatalf("get old pod: %v", err)
	}
	if pod.Labels[LabelReady] != "true" {
		t.Errorf("old pod ready label = %q, want %q", pod.Labels[LabelReady], "true")
	}

	// Annotations should be cleared.
	var updated chainsv1alpha2.ChainInstance
	err = k.Get(ctx, types.NamespacedName{Name: "eth-node", Namespace: "default"}, &updated)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if _, ok := updated.Annotations[AnnotationReplacementPhase]; ok {
		t.Error("replacement phase annotation should be cleared after rollback")
	}
}

func TestIsReplacementInProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty phase",
			annotations: map[string]string{AnnotationReplacementPhase: ""},
			want:        false,
		},
		{
			name:        "completing phase is not in progress",
			annotations: map[string]string{AnnotationReplacementPhase: string(ReplacementCompleting)},
			want:        false,
		},
		{
			name:        "draining phase is in progress",
			annotations: map[string]string{AnnotationReplacementPhase: string(ReplacementDraining)},
			want:        true,
		},
		{
			name:        "verifying phase is in progress",
			annotations: map[string]string{AnnotationReplacementPhase: string(ReplacementVerifying)},
			want:        true,
		},
		{
			name:        "creating phase is in progress",
			annotations: map[string]string{AnnotationReplacementPhase: string(ReplacementCreating)},
			want:        true,
		},
		{
			name:        "rolling back is in progress",
			annotations: map[string]string{AnnotationReplacementPhase: string(ReplacementRollingBack)},
			want:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			node := &chainsv1alpha2.ChainInstance{
				ObjectMeta: metav1.ObjectMeta{Annotations: tc.annotations},
			}
			if got := IsReplacementInProgress(node); got != tc.want {
				t.Errorf("IsReplacementInProgress() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetReplacementPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		want        ReplacementPhase
	}{
		{
			name:        "no annotations returns empty",
			annotations: nil,
			want:        "",
		},
		{
			name:        "returns current phase",
			annotations: map[string]string{AnnotationReplacementPhase: string(ReplacementVerifying)},
			want:        ReplacementVerifying,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			node := &chainsv1alpha2.ChainInstance{
				ObjectMeta: metav1.ObjectMeta{Annotations: tc.annotations},
			}
			if got := GetReplacementPhase(node); got != tc.want {
				t.Errorf("GetReplacementPhase() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultReplacementConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultReplacementConfig()
	if cfg.DrainTimeout != 30*time.Second {
		t.Errorf("DrainTimeout = %v, want 30s", cfg.DrainTimeout)
	}
	if cfg.SyncTimeout != 10*time.Minute {
		t.Errorf("SyncTimeout = %v, want 10m", cfg.SyncTimeout)
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("PollInterval = %v, want 10s", cfg.PollInterval)
	}
	if cfg.GracePeriodSeconds != 30 {
		t.Errorf("GracePeriodSeconds = %d, want 30", cfg.GracePeriodSeconds)
	}
	if cfg.CleanupPVC {
		t.Error("CleanupPVC should be false by default")
	}
}

func TestContainersReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		statuses  []corev1.ContainerStatus
		wantReady bool
	}{
		{
			name:      "no container statuses",
			statuses:  nil,
			wantReady: false,
		},
		{
			name: "all ready",
			statuses: []corev1.ContainerStatus{
				{Name: "a", Ready: true},
				{Name: "b", Ready: true},
			},
			wantReady: true,
		},
		{
			name: "one not ready",
			statuses: []corev1.ContainerStatus{
				{Name: "a", Ready: true},
				{Name: "b", Ready: false},
			},
			wantReady: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pod := &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: tc.statuses}}
			if got := containersReady(pod); got != tc.wantReady {
				t.Errorf("containersReady() = %v, want %v", got, tc.wantReady)
			}
		})
	}
}

func TestCloneLabels(t *testing.T) {
	t.Parallel()

	src := map[string]string{"a": "1", "b": "2"}
	dst := cloneLabels(src)

	if len(dst) != 2 {
		t.Errorf("cloneLabels length = %d, want 2", len(dst))
	}

	dst["c"] = "3"
	if _, ok := src["c"]; ok {
		t.Error("cloneLabels should not modify source")
	}
}

func TestNodeAnnotation(t *testing.T) {
	t.Parallel()

	t.Run("nil annotations", func(t *testing.T) {
		t.Parallel()
		node := &chainsv1alpha2.ChainInstance{}
		if got := nodeAnnotation(node, "key"); got != "" {
			t.Errorf("nodeAnnotation on nil = %q, want empty", got)
		}
	})

	t.Run("existing key", func(t *testing.T) {
		t.Parallel()
		node := &chainsv1alpha2.ChainInstance{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"key": "value"},
			},
		}
		if got := nodeAnnotation(node, "key"); got != "value" {
			t.Errorf("nodeAnnotation = %q, want %q", got, "value")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		t.Parallel()
		node := &chainsv1alpha2.ChainInstance{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"other": "value"},
			},
		}
		if got := nodeAnnotation(node, "key"); got != "" {
			t.Errorf("nodeAnnotation = %q, want empty", got)
		}
	})
}

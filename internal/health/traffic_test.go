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
)

func trafficTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	return s
}

func newTrafficManager(objs ...client.Object) (*LabelBasedTrafficManager, client.Client) {
	s := trafficTestScheme()
	k := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return NewLabelBasedTrafficManager(k), k
}

func trafficPod(name, ns string, labels map[string]string, ip string) *corev1.Pod {
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
		Status: corev1.PodStatus{
			PodIP: ip,
		},
	}
}

func TestDrain_SetsReadyFalse(t *testing.T) {
	t.Parallel()

	pod := trafficPod("eth-0", "default", map[string]string{
		"app":      "eth",
		LabelReady: "true",
	}, "10.0.0.1")

	tm, k := newTrafficManager(pod)

	// Drain waits for the remaining context duration (up to defaultDrainTimeout).
	// Use a very short timeout so the wait completes quickly. The label is set
	// before the wait begins.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Drain returns ctx.Err() when the wait finishes via context expiry;
	// that is expected behavior, not a test failure.
	_ = tm.Drain(ctx, "eth-0", "default")

	var updated corev1.Pod
	if err := k.Get(ctx, types.NamespacedName{Name: "eth-0", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if updated.Labels[LabelReady] != "false" {
		t.Errorf("ready label = %q, want %q", updated.Labels[LabelReady], "false")
	}
}

func TestDrain_PodNotFound(t *testing.T) {
	t.Parallel()

	tm, _ := newTrafficManager() // no pods

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := tm.Drain(ctx, "missing-pod", "default")
	if err != nil {
		t.Fatalf("Drain on missing pod should return nil, got: %v", err)
	}
}

func TestSwitchTraffic_UpdatesBothPods(t *testing.T) {
	t.Parallel()

	oldPod := trafficPod("eth-0", "default", map[string]string{
		"app":      "eth",
		LabelReady: "true",
	}, "10.0.0.1")

	newPod := trafficPod("eth-0-replacement", "default", map[string]string{
		"app":         "eth",
		LabelReady:    "false",
		LabelInstance: "replacement",
	}, "10.0.0.2")

	tm, k := newTrafficManager(oldPod, newPod)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := tm.SwitchTraffic(ctx, "eth-0", "eth-0-replacement", "default")
	if err != nil {
		t.Fatalf("SwitchTraffic failed: %v", err)
	}

	// Old pod should have ready=false.
	var old corev1.Pod
	if err := k.Get(ctx, types.NamespacedName{Name: "eth-0", Namespace: "default"}, &old); err != nil {
		t.Fatalf("get old pod: %v", err)
	}
	if old.Labels[LabelReady] != "false" {
		t.Errorf("old pod ready = %q, want %q", old.Labels[LabelReady], "false")
	}

	// New pod should have ready=true.
	var np corev1.Pod
	if err := k.Get(ctx, types.NamespacedName{Name: "eth-0-replacement", Namespace: "default"}, &np); err != nil {
		t.Fatalf("get new pod: %v", err)
	}
	if np.Labels[LabelReady] != "true" {
		t.Errorf("new pod ready = %q, want %q", np.Labels[LabelReady], "true")
	}
}

func TestSwitchTraffic_OldPodNotFound(t *testing.T) {
	t.Parallel()

	newPod := trafficPod("eth-0-replacement", "default", map[string]string{
		"app":      "eth",
		LabelReady: "false",
	}, "10.0.0.2")

	tm, _ := newTrafficManager(newPod)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := tm.SwitchTraffic(ctx, "eth-0", "eth-0-replacement", "default")
	if err != nil {
		t.Fatalf("SwitchTraffic should succeed when old pod missing: %v", err)
	}
}

func TestSwitchTraffic_ContextCancelled(t *testing.T) {
	t.Parallel()

	oldPod := trafficPod("eth-0", "default", map[string]string{LabelReady: "true"}, "10.0.0.1")
	newPod := trafficPod("eth-0-rep", "default", map[string]string{LabelReady: "false"}, "10.0.0.2")

	tm, _ := newTrafficManager(oldPod, newPod)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := tm.SwitchTraffic(ctx, "eth-0", "eth-0-rep", "default")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestValidateTraffic_ReadyLabel(t *testing.T) {
	t.Parallel()

	t.Run("pod with ready=true and no matching services", func(t *testing.T) {
		t.Parallel()
		pod := trafficPod("eth-0", "default", map[string]string{
			"app":      "eth",
			LabelReady: "true",
		}, "10.0.0.1")

		tm, _ := newTrafficManager(pod)
		ok, err := tm.ValidateTraffic(context.Background(), "eth-0", "default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Error("expected validation to pass with ready=true and no services")
		}
	})

	t.Run("pod with ready=false", func(t *testing.T) {
		t.Parallel()
		pod := trafficPod("eth-0", "default", map[string]string{
			"app":      "eth",
			LabelReady: "false",
		}, "10.0.0.1")

		tm, _ := newTrafficManager(pod)
		ok, err := tm.ValidateTraffic(context.Background(), "eth-0", "default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected validation to fail with ready=false")
		}
	})

	t.Run("pod missing ready label", func(t *testing.T) {
		t.Parallel()
		pod := trafficPod("eth-0", "default", map[string]string{"app": "eth"}, "10.0.0.1")

		tm, _ := newTrafficManager(pod)
		ok, err := tm.ValidateTraffic(context.Background(), "eth-0", "default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected validation to fail without ready label")
		}
	})

	t.Run("pod with no IP", func(t *testing.T) {
		t.Parallel()
		pod := trafficPod("eth-0", "default", map[string]string{
			LabelReady: "true",
		}, "")

		tm, _ := newTrafficManager(pod)
		ok, err := tm.ValidateTraffic(context.Background(), "eth-0", "default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected validation to fail without pod IP")
		}
	})
}

func TestValidateTraffic_WithEndpoints(t *testing.T) {
	t.Parallel()

	pod := trafficPod("eth-0", "default", map[string]string{
		"app":      "eth",
		LabelReady: "true",
	}, "10.0.0.5")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eth-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "eth", LabelReady: "true"},
		},
	}

	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eth-svc",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "10.0.0.5"},
				},
			},
		},
	}

	tm, _ := newTrafficManager(pod, svc, ep)
	ok, err := tm.ValidateTraffic(context.Background(), "eth-0", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected validation to pass with matching endpoint")
	}
}

func TestValidateTraffic_PodNotInEndpoints(t *testing.T) {
	t.Parallel()

	pod := trafficPod("eth-0", "default", map[string]string{
		"app":      "eth",
		LabelReady: "true",
	}, "10.0.0.5")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eth-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "eth", LabelReady: "true"},
		},
	}

	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eth-svc",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "10.0.0.99"}, // different IP
				},
			},
		},
	}

	tm, _ := newTrafficManager(pod, svc, ep)
	ok, err := tm.ValidateTraffic(context.Background(), "eth-0", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected validation to fail when pod IP not in endpoints")
	}
}

func TestValidateTraffic_PodNotFound(t *testing.T) {
	t.Parallel()

	tm, _ := newTrafficManager() // no pods
	_, err := tm.ValidateTraffic(context.Background(), "missing", "default")
	if err == nil {
		t.Error("expected error for missing pod")
	}
}

func TestLabelsContain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		labels   map[string]string
		required map[string]string
		want     bool
	}{
		{
			name:     "all required present",
			labels:   map[string]string{"a": "1", "b": "2", "c": "3"},
			required: map[string]string{"a": "1", "b": "2"},
			want:     true,
		},
		{
			name:     "missing key",
			labels:   map[string]string{"a": "1"},
			required: map[string]string{"a": "1", "b": "2"},
			want:     false,
		},
		{
			name:     "wrong value",
			labels:   map[string]string{"a": "1", "b": "3"},
			required: map[string]string{"a": "1", "b": "2"},
			want:     false,
		},
		{
			name:     "empty required",
			labels:   map[string]string{"a": "1"},
			required: map[string]string{},
			want:     true,
		},
		{
			name:     "nil labels",
			labels:   nil,
			required: map[string]string{"a": "1"},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := labelsContain(tc.labels, tc.required); got != tc.want {
				t.Errorf("labelsContain() = %v, want %v", got, tc.want)
			}
		})
	}
}

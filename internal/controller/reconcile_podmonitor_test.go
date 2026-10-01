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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

func podMonitorTestNode() *chainsv1alpha2.ChainInstance {
	node := &chainsv1alpha2.ChainInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "eth-0", Namespace: "chains"},
	}
	node.Spec.Chain = chainsv1alpha2.ChainEthereum
	node.Spec.Monitoring.PodMonitor.Interval = "30s"
	return node
}

func TestApplyPodMonitorSpec_SetsSelectorAndEndpoints(t *testing.T) {
	pm := &unstructured.Unstructured{Object: map[string]any{}}
	if err := applyPodMonitorSpec(pm, podMonitorTestNode()); err != nil {
		t.Fatalf("applyPodMonitorSpec: %v", err)
	}

	app, found, err := unstructured.NestedString(pm.Object, "spec", "selector", "matchLabels", "app")
	if err != nil || !found || app != "eth-0" {
		t.Errorf("matchLabels.app = %q (found=%v, err=%v), want eth-0", app, found, err)
	}
	endpoints, found, err := unstructured.NestedSlice(pm.Object, "spec", "podMetricsEndpoints")
	if err != nil || !found || len(endpoints) != 1 {
		t.Fatalf("podMetricsEndpoints = %v (found=%v, err=%v), want one endpoint", endpoints, found, err)
	}
	if ep := endpoints[0].(map[string]any); ep["interval"] != "30s" {
		t.Errorf("endpoint interval = %v, want 30s", ep["interval"])
	}
}

func TestApplyPodMonitorSpec_NonMapSpecReturnsError(t *testing.T) {
	pm := &unstructured.Unstructured{Object: map[string]any{"spec": "not-a-map"}}
	if err := applyPodMonitorSpec(pm, podMonitorTestNode()); err == nil {
		t.Fatal("expected error when spec is not a map, got nil")
	}
}

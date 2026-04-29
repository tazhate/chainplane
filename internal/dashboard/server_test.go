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
package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha2.AddToScheme(s)
	return s
}

func buildServer(objects ...runtime.Object) *Server {
	cl := fake.NewClientBuilder().
		WithScheme(newScheme()).
		WithRuntimeObjects(objects...).
		Build()
	return New(Config{
		Client:    cl,
		Namespace: "",
		Refresh:   time.Minute,
	})
}

func makeNode(ns, name string, chain v1alpha2.Chain, phase v1alpha2.NodePhase) *v1alpha2.ChainInstance {
	return &v1alpha2.ChainInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: metav1.Now(),
		},
		Spec: v1alpha2.ChainInstanceSpec{
			Chain:   chain,
			Network: v1alpha2.NetworkMainnet,
		},
		Status: v1alpha2.ChainInstanceStatus{
			Phase: phase,
		},
	}
}

// TestListNodes_Empty checks that an empty cluster returns zeroed summary.
func TestListNodes_Empty(t *testing.T) {
	s := buildServer()
	s.refreshCache(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	s.handleNodeList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp NodeListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Summary.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Summary.Total)
	}
	if resp.Summary.Healthy != 0 || resp.Summary.Syncing != 0 || resp.Summary.Failed != 0 {
		t.Errorf("unexpected non-zero summary: %+v", resp.Summary)
	}
}

// TestListNodes_MixedPhases verifies summary counts with 3 nodes of different phases.
func TestListNodes_MixedPhases(t *testing.T) {
	nodes := []runtime.Object{
		makeNode("default", "eth-1", v1alpha2.ChainEthereum, v1alpha2.NodePhaseHealthy),
		makeNode("default", "eth-2", v1alpha2.ChainEthereum, v1alpha2.NodePhaseSyncing),
		makeNode("default", "btc-1", v1alpha2.ChainBitcoin, v1alpha2.NodePhaseDegraded),
	}

	s := buildServer(nodes...)
	s.refreshCache(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	s.handleNodeList(w, req)

	var resp NodeListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Summary.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Summary.Total)
	}
	if resp.Summary.Healthy != 1 {
		t.Errorf("expected healthy=1, got %d", resp.Summary.Healthy)
	}
	if resp.Summary.Syncing != 1 {
		t.Errorf("expected syncing=1, got %d", resp.Summary.Syncing)
	}
	if resp.Summary.Degraded != 1 {
		t.Errorf("expected degraded=1, got %d", resp.Summary.Degraded)
	}
}

// TestListNodes_Namespace verifies namespace filtering.
func TestListNodes_Namespace(t *testing.T) {
	nodes := []runtime.Object{
		makeNode("ns-a", "eth-1", v1alpha2.ChainEthereum, v1alpha2.NodePhaseHealthy),
		makeNode("ns-b", "eth-2", v1alpha2.ChainEthereum, v1alpha2.NodePhaseHealthy),
	}

	cl := fake.NewClientBuilder().
		WithScheme(newScheme()).
		WithRuntimeObjects(nodes...).
		Build()

	s := New(Config{Client: cl, Namespace: "ns-a", Refresh: time.Minute})
	s.refreshCache(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	s.handleNodeList(w, req)

	var resp NodeListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Summary.Total != 1 {
		t.Errorf("expected total=1 with ns filter, got %d", resp.Summary.Total)
	}
	if resp.Nodes[0].Namespace != "ns-a" {
		t.Errorf("expected ns-a, got %s", resp.Nodes[0].Namespace)
	}
}

// TestMetricsEndpoint verifies that /metrics returns bch_dashboard_nodes_total.
func TestMetricsEndpoint(t *testing.T) {
	nodes := []runtime.Object{
		makeNode("default", "eth-1", v1alpha2.ChainEthereum, v1alpha2.NodePhaseHealthy),
	}
	s := buildServer(nodes...)
	s.refreshCache(context.Background())

	handler := s.Handler(http.Dir("."))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "bch_dashboard_nodes_total") {
		t.Errorf("expected bch_dashboard_nodes_total in metrics output")
	}
}

// TestHealthEndpoint verifies GET /healthz returns "ok".
func TestHealthEndpoint(t *testing.T) {
	s := buildServer()
	handler := s.Handler(http.Dir("."))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "ok" {
		t.Errorf("expected 'ok', got %q", body)
	}
}

// TestNodeInfoAge checks that Age is computed from CreationTimestamp.
func TestNodeInfoAge(t *testing.T) {
	node := &v1alpha2.ChainInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "eth-old",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-72 * time.Hour)),
		},
		Spec: v1alpha2.ChainInstanceSpec{
			Chain:   v1alpha2.ChainEthereum,
			Network: v1alpha2.NetworkMainnet,
		},
	}
	info := nodeToInfo(node)
	if !strings.HasSuffix(info.Age, "d") {
		t.Errorf("expected age in days (e.g. '3d'), got %q", info.Age)
	}
}

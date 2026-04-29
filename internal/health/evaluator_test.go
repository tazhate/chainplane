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
	"errors"
	"testing"
	"time"
)

// mockQuerier implements MetricQuerier for testing.
type mockQuerier struct {
	healthy       bool
	instantValues map[string]float64
	instantErrors map[string]error
	sustainedMap  map[string]bool
	sustainedErr  map[string]error
	rangeValues   map[string][]MatrixSeries
	rangeErrors   map[string]error
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{
		healthy:       true,
		instantValues: make(map[string]float64),
		instantErrors: make(map[string]error),
		sustainedMap:  make(map[string]bool),
		sustainedErr:  make(map[string]error),
		rangeValues:   make(map[string][]MatrixSeries),
		rangeErrors:   make(map[string]error),
	}
}

func (m *mockQuerier) Healthy(_ context.Context) bool { return m.healthy }

func (m *mockQuerier) InstantValue(_ context.Context, promql string) (float64, error) {
	if err, ok := m.instantErrors[promql]; ok {
		return 0, err
	}
	// Also check for a wildcard match.
	for pattern, err := range m.instantErrors {
		if pattern == "*" {
			return 0, err
		}
	}
	if val, ok := m.instantValues[promql]; ok {
		return val, nil
	}
	for pattern, val := range m.instantValues {
		if pattern == "*" {
			return val, nil
		}
	}
	return 0, &QueryError{Query: promql, Err: ErrNoMetricData}
}

func (m *mockQuerier) Sustained(_ context.Context, promql string, _ float64, _ int) (bool, error) {
	if err, ok := m.sustainedErr[promql]; ok {
		return false, err
	}
	for pattern, err := range m.sustainedErr {
		if pattern == "*" {
			return false, err
		}
	}
	if val, ok := m.sustainedMap[promql]; ok {
		return val, nil
	}
	for pattern, val := range m.sustainedMap {
		if pattern == "*" {
			return val, nil
		}
	}
	return false, nil
}

func (m *mockQuerier) RangeValues(_ context.Context, promql string, _, _ time.Time, _ string) ([]MatrixSeries, error) {
	if err, ok := m.rangeErrors[promql]; ok {
		return nil, err
	}
	for pattern, err := range m.rangeErrors {
		if pattern == "*" {
			return nil, err
		}
	}
	if val, ok := m.rangeValues[promql]; ok {
		return val, nil
	}
	for pattern, val := range m.rangeValues {
		if pattern == "*" {
			return val, nil
		}
	}
	return nil, nil
}

func TestChecker_CheckAll_Healthy(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	// Set all metrics to healthy values.
	mq.instantValues["*"] = 0
	mq.sustainedMap["*"] = false

	// Need a valid range value for crash loop check.
	valBytes, _ := json.Marshal("0")
	tsBytes, _ := json.Marshal(float64(time.Now().Unix()))
	mq.rangeValues["*"] = []MatrixSeries{{
		Values: [][2]json.RawMessage{{tsBytes, valBytes}},
	}}

	c := NewChecker(mq, Thresholds{})
	status := c.CheckAll(context.Background(), "eth-node-0", BlockchainEthereum)

	if !status.Healthy() {
		t.Errorf("expected healthy status, got triggers: %v", status.TriggeredNames())
	}
	if status.PodName != "eth-node-0" {
		t.Errorf("PodName = %q, want %q", status.PodName, "eth-node-0")
	}
	if status.Blockchain != BlockchainEthereum {
		t.Errorf("Blockchain = %q, want %q", status.Blockchain, BlockchainEthereum)
	}
	if len(status.Results) != len(allTriggers) {
		t.Errorf("Results has %d entries, want %d", len(status.Results), len(allTriggers))
	}
}

func TestChecker_CheckAll_UnhealthyBackend(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.healthy = false

	c := NewChecker(mq, Thresholds{})
	status := c.CheckAll(context.Background(), "pod-0", BlockchainEthereum)

	if !status.Healthy() {
		t.Error("expected healthy status when backend is unreachable (safe fallback)")
	}
	for _, name := range allTriggers {
		r, ok := status.Results[name]
		if !ok {
			t.Errorf("missing result for trigger %s", name)
			continue
		}
		if r.Value != "0" {
			t.Errorf("trigger %s: Value = %q, want %q", name, r.Value, "0")
		}
		if r.Threshold != "n/a" {
			t.Errorf("trigger %s: Threshold = %q, want %q", name, r.Threshold, "n/a")
		}
	}
}

func TestChecker_EvalSyncLag_Triggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	query := `eth_sync_lag{node="pod-0"}`
	mq.instantValues[query] = 100 // above threshold of 30
	mq.sustainedMap[query] = true

	c := NewChecker(mq, Thresholds{})
	result := c.evalSyncLag(context.Background(), "pod-0", BlockchainEthereum)

	if !result.Fired {
		t.Error("expected sync lag trigger to fire")
	}
	if result.Value != "100" {
		t.Errorf("Value = %q, want %q", result.Value, "100")
	}
}

func TestChecker_EvalSyncLag_NotTriggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	query := `eth_sync_lag{node="pod-0"}`
	mq.instantValues[query] = 10 // below threshold
	mq.sustainedMap[query] = false

	c := NewChecker(mq, Thresholds{})
	result := c.evalSyncLag(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected sync lag trigger NOT to fire")
	}
}

func TestChecker_EvalSyncLag_NoData(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	query := `eth_sync_lag{node="pod-0"}`
	mq.instantErrors[query] = &QueryError{Query: query, Err: ErrNoMetricData}

	c := NewChecker(mq, Thresholds{})
	result := c.evalSyncLag(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected sync lag NOT to fire on no data")
	}
	if result.Value != "0" {
		t.Errorf("Value = %q, want %q", result.Value, "0")
	}
}

func TestChecker_EvalSyncLag_Solana(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	query := `sol_slot_lag{node="sol-0"}`
	mq.instantValues[query] = 200
	mq.sustainedMap[query] = true

	c := NewChecker(mq, Thresholds{})
	result := c.evalSyncLag(context.Background(), "sol-0", BlockchainSolana)

	if !result.Fired {
		t.Error("expected solana sync lag trigger to fire")
	}
	if result.Threshold != "150" {
		t.Errorf("Threshold = %q, want %q (solana default)", result.Threshold, "150")
	}
}

func TestChecker_EvalErrorRate_Triggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	// The error rate query is dynamically built.
	mq.instantValues["*"] = 0.1 // above 0.05 threshold

	c := NewChecker(mq, Thresholds{})
	result := c.evalErrorRate(context.Background(), "pod-0", BlockchainEthereum)

	if !result.Fired {
		t.Error("expected error rate trigger to fire")
	}
}

func TestChecker_EvalErrorRate_BelowThreshold(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantValues["*"] = 0.01

	c := NewChecker(mq, Thresholds{})
	result := c.evalErrorRate(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected error rate trigger NOT to fire")
	}
}

func TestChecker_EvalLatency_Triggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantValues["*"] = 5.0
	mq.sustainedMap["*"] = true

	c := NewChecker(mq, Thresholds{})
	result := c.evalLatency(context.Background(), "pod-0", BlockchainEthereum)

	if !result.Fired {
		t.Error("expected latency trigger to fire")
	}
	if result.Threshold != "2" {
		t.Errorf("Threshold = %q, want %q (ethereum default)", result.Threshold, "2")
	}
}

func TestChecker_EvalLatency_Solana(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantValues["*"] = 1.0
	mq.sustainedMap["*"] = true

	c := NewChecker(mq, Thresholds{})
	result := c.evalLatency(context.Background(), "sol-0", BlockchainSolana)

	if !result.Fired {
		t.Error("expected latency trigger to fire for solana")
	}
	if result.Threshold != "0.5" {
		t.Errorf("Threshold = %q, want %q (solana default)", result.Threshold, "0.5")
	}
}

func TestChecker_EvalCrashLoop_Triggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	// Current restart count = 10.
	mq.instantValues["*"] = 10
	// Past restart count = 5, so restarts in window = 5 >= threshold 3.
	valBytes, _ := json.Marshal("5")
	tsBytes, _ := json.Marshal(float64(time.Now().Add(-10 * time.Minute).Unix()))
	mq.rangeValues["*"] = []MatrixSeries{{
		Values: [][2]json.RawMessage{{tsBytes, valBytes}},
	}}

	c := NewChecker(mq, Thresholds{})
	result := c.evalCrashLoop(context.Background(), "pod-0", BlockchainEthereum)

	if !result.Fired {
		t.Error("expected crash loop trigger to fire")
	}
}

func TestChecker_EvalCrashLoop_NotTriggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantValues["*"] = 2
	// Past restarts = 1, so restarts in window = 1 < 3.
	valBytes, _ := json.Marshal("1")
	tsBytes, _ := json.Marshal(float64(time.Now().Add(-10 * time.Minute).Unix()))
	mq.rangeValues["*"] = []MatrixSeries{{
		Values: [][2]json.RawMessage{{tsBytes, valBytes}},
	}}

	c := NewChecker(mq, Thresholds{})
	result := c.evalCrashLoop(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected crash loop trigger NOT to fire")
	}
}

func TestChecker_EvalDiskUsage_Triggered(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantValues["*"] = 0.95 // above 0.9

	c := NewChecker(mq, Thresholds{})
	result := c.evalDiskUsage(context.Background(), "pod-0", BlockchainEthereum)

	if !result.Fired {
		t.Error("expected disk usage trigger to fire")
	}
}

func TestChecker_EvalDiskUsage_BelowThreshold(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantValues["*"] = 0.5

	c := NewChecker(mq, Thresholds{})
	result := c.evalDiskUsage(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected disk usage trigger NOT to fire")
	}
}

func TestChecker_CheckAll_MixedTriggers(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	// Sync lag: healthy.
	syncQuery := `eth_sync_lag{node="pod-0"}`
	mq.instantValues[syncQuery] = 5
	mq.sustainedMap[syncQuery] = false
	// All other instant queries return high values to trigger.
	mq.instantValues["*"] = 0.95
	mq.sustainedMap["*"] = true

	valBytes, _ := json.Marshal("0")
	tsBytes, _ := json.Marshal(float64(time.Now().Unix()))
	mq.rangeValues["*"] = []MatrixSeries{{
		Values: [][2]json.RawMessage{{tsBytes, valBytes}},
	}}

	c := NewChecker(mq, Thresholds{})
	status := c.CheckAll(context.Background(), "pod-0", BlockchainEthereum)

	if status.Healthy() {
		t.Error("expected unhealthy status with mixed triggers")
	}

	triggered := status.TriggeredNames()
	if len(triggered) == 0 {
		t.Error("expected at least one triggered name")
	}
}

func TestChecker_SafeEval_PanicRecovery(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	c := NewChecker(mq, Thresholds{})

	ev := triggerEval{
		name: TriggerSyncLag,
		fn: func(_ context.Context, _ string, _ BlockchainType) TriggerResult {
			panic("test panic")
		},
	}

	result := c.safeEval(context.Background(), ev, "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("panicked trigger should not fire")
	}
	if result.Value != "0" {
		t.Errorf("Value = %q, want %q", result.Value, "0")
	}
	if result.Threshold != "n/a" {
		t.Errorf("Threshold = %q, want %q", result.Threshold, "n/a")
	}
}

func TestChecker_EvalSyncLag_UnknownChain(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	c := NewChecker(mq, Thresholds{})

	result := c.evalSyncLag(context.Background(), "pod-0", BlockchainType("unknown"))

	if result.Fired {
		t.Error("unknown chain should not fire")
	}
	if result.Threshold != "n/a" {
		t.Errorf("Threshold = %q, want %q for unknown chain", result.Threshold, "n/a")
	}
}

func TestChecker_EvalErrorRate_NoData(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantErrors["*"] = &QueryError{Query: "test", Err: ErrNoMetricData}

	c := NewChecker(mq, Thresholds{})
	result := c.evalErrorRate(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected error rate NOT to fire on no data")
	}
}

func TestChecker_EvalDiskUsage_QueryError(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	mq.instantErrors["*"] = &QueryError{Query: "test", Err: errors.New("connection refused")}

	c := NewChecker(mq, Thresholds{})
	result := c.evalDiskUsage(context.Background(), "pod-0", BlockchainEthereum)

	if result.Fired {
		t.Error("expected disk usage NOT to fire on query error")
	}
}

func TestChecker_CustomThresholds(t *testing.T) {
	t.Parallel()

	mq := newMockQuerier()
	// Value is 0.08, above custom threshold of 0.03 but below default 0.05.
	mq.instantValues["*"] = 0.08

	c := NewChecker(mq, Thresholds{ErrorRate: 0.03})
	result := c.evalErrorRate(context.Background(), "pod-0", BlockchainEthereum)

	if !result.Fired {
		t.Error("expected error rate to fire with custom lower threshold")
	}
	if result.Threshold != "0.03" {
		t.Errorf("Threshold = %q, want %q", result.Threshold, "0.03")
	}
}

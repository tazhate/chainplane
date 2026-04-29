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
	"strings"
	"testing"
)

func TestTriggerResult(t *testing.T) {
	t.Parallel()

	t.Run("fired trigger", func(t *testing.T) {
		t.Parallel()
		r := TriggerResult{Fired: true, Value: "50", Threshold: "30"}
		if !r.Fired {
			t.Error("expected Fired=true")
		}
	})

	t.Run("not fired trigger", func(t *testing.T) {
		t.Parallel()
		r := TriggerResult{Fired: false, Value: "10", Threshold: "30"}
		if r.Fired {
			t.Error("expected Fired=false")
		}
	})
}

func TestHealthStatus_Healthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		results map[TriggerName]TriggerResult
		want    bool
	}{
		{
			name:    "all triggers ok",
			results: map[TriggerName]TriggerResult{TriggerSyncLag: {Fired: false}, TriggerErrorRate: {Fired: false}},
			want:    true,
		},
		{
			name:    "one trigger fired",
			results: map[TriggerName]TriggerResult{TriggerSyncLag: {Fired: true}, TriggerErrorRate: {Fired: false}},
			want:    false,
		},
		{
			name:    "all triggers fired",
			results: map[TriggerName]TriggerResult{TriggerSyncLag: {Fired: true}, TriggerErrorRate: {Fired: true}},
			want:    false,
		},
		{
			name:    "empty results is healthy",
			results: map[TriggerName]TriggerResult{},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := HealthStatus{PodName: "test-0", Results: tc.results}
			if got := s.Healthy(); got != tc.want {
				t.Errorf("Healthy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHealthStatus_TriggeredNames(t *testing.T) {
	t.Parallel()

	s := HealthStatus{
		PodName: "test-0",
		Results: map[TriggerName]TriggerResult{
			TriggerSyncLag:   {Fired: true},
			TriggerErrorRate: {Fired: false},
			TriggerLatency:   {Fired: true},
			TriggerCrashLoop: {Fired: false},
			TriggerDiskUsage: {Fired: false},
		},
	}

	names := s.TriggeredNames()
	if len(names) != 2 {
		t.Fatalf("TriggeredNames() returned %d names, want 2", len(names))
	}

	nameSet := make(map[TriggerName]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet[TriggerSyncLag] {
		t.Error("expected TriggerSyncLag in triggered names")
	}
	if !nameSet[TriggerLatency] {
		t.Error("expected TriggerLatency in triggered names")
	}
}

func TestHealthStatus_Summary(t *testing.T) {
	t.Parallel()

	s := HealthStatus{
		PodName:    "eth-node-0",
		Blockchain: BlockchainEthereum,
		Results: map[TriggerName]TriggerResult{
			TriggerSyncLag:   {Fired: true, Value: "50", Threshold: "30"},
			TriggerErrorRate: {Fired: false, Value: "0.01", Threshold: "0.05"},
			TriggerLatency:   {Fired: false, Value: "1.5", Threshold: "2"},
			TriggerCrashLoop: {Fired: false, Value: "0", Threshold: "3"},
			TriggerDiskUsage: {Fired: false, Value: "0.5", Threshold: "0.9"},
		},
	}

	summary := s.Summary()

	if !strings.Contains(summary, "TRIGGERED") {
		t.Error("summary should contain TRIGGERED for fired trigger")
	}
	if !strings.Contains(summary, "sync_lag") {
		t.Error("summary should contain trigger name sync_lag")
	}
	if !strings.Contains(summary, "Total triggers active: 1/5") {
		t.Errorf("summary should contain correct trigger count, got:\n%s", summary)
	}
}

func TestAllTriggers(t *testing.T) {
	t.Parallel()

	triggers := AllTriggers()
	if len(triggers) != 5 {
		t.Fatalf("AllTriggers() returned %d, want 5", len(triggers))
	}

	// Verify the returned slice is a copy.
	triggers[0] = "mutated"
	original := AllTriggers()
	if original[0] == "mutated" {
		t.Error("AllTriggers should return a copy, not a reference to the original")
	}
}

func TestDefaultThresholds(t *testing.T) {
	t.Parallel()

	d := DefaultThresholds()

	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"SyncLagETH", d.SyncLagETH, 30},
		{"SyncLagSOL", d.SyncLagSOL, 150},
		{"ErrorRate", d.ErrorRate, 0.05},
		{"LatencyETH", d.LatencyETH, 2.0},
		{"LatencySOL", d.LatencySOL, 0.5},
		{"DiskUsage", d.DiskUsage, 0.9},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	intChecks := []struct {
		name string
		got  int
		want int
	}{
		{"SyncLagDuration", d.SyncLagDuration, 10},
		{"ErrorRateWindow", d.ErrorRateWindow, 5},
		{"LatencyDuration", d.LatencyDuration, 5},
		{"CrashLoopRestarts", d.CrashLoopRestarts, 3},
		{"CrashLoopWindow", d.CrashLoopWindow, 10},
	}
	for _, c := range intChecks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestThresholds_applyDefaults(t *testing.T) {
	t.Parallel()

	t.Run("zero value gets defaults", func(t *testing.T) {
		t.Parallel()
		var zero Thresholds
		got := zero.applyDefaults()
		want := DefaultThresholds()
		if got != want {
			t.Errorf("applyDefaults() on zero = %+v, want %+v", got, want)
		}
	})

	t.Run("custom values preserved", func(t *testing.T) {
		t.Parallel()
		custom := Thresholds{
			SyncLagETH: 100,
			ErrorRate:  0.1,
		}
		got := custom.applyDefaults()
		if got.SyncLagETH != 100 {
			t.Errorf("SyncLagETH = %v, want 100", got.SyncLagETH)
		}
		if got.ErrorRate != 0.1 {
			t.Errorf("ErrorRate = %v, want 0.1", got.ErrorRate)
		}
		// Unset fields should get defaults.
		if got.SyncLagSOL != 150 {
			t.Errorf("SyncLagSOL = %v, want 150 (default)", got.SyncLagSOL)
		}
		if got.LatencyDuration != 5 {
			t.Errorf("LatencyDuration = %v, want 5 (default)", got.LatencyDuration)
		}
	})
}

func TestThresholds_syncLagThreshold(t *testing.T) {
	t.Parallel()

	th := DefaultThresholds()

	tests := []struct {
		name  string
		chain BlockchainType
		want  float64
	}{
		{"ethereum", BlockchainEthereum, 30},
		{"solana", BlockchainSolana, 150},
		{"unknown defaults to ETH", BlockchainType("unknown"), 30},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := th.syncLagThreshold(tc.chain); got != tc.want {
				t.Errorf("syncLagThreshold(%q) = %v, want %v", tc.chain, got, tc.want)
			}
		})
	}
}

func TestThresholds_latencyThreshold(t *testing.T) {
	t.Parallel()

	th := DefaultThresholds()

	tests := []struct {
		name  string
		chain BlockchainType
		want  float64
	}{
		{"ethereum", BlockchainEthereum, 2.0},
		{"solana", BlockchainSolana, 0.5},
		{"unknown defaults to ETH", BlockchainType("unknown"), 2.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := th.latencyThreshold(tc.chain); got != tc.want {
				t.Errorf("latencyThreshold(%q) = %v, want %v", tc.chain, got, tc.want)
			}
		})
	}
}

func TestFormatMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0"},
		{1.0, "1"},
		{0.05, "0.05"},
		{2.1234, "2.1234"},
		{100.10, "100.1"},
		{0.00010, "0.0001"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := formatMetric(tc.input); got != tc.want {
				t.Errorf("formatMetric(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNoDataResult(t *testing.T) {
	t.Parallel()

	r := noDataResult("30")
	if r.Fired {
		t.Error("noDataResult should not be fired")
	}
	if r.Value != "0" {
		t.Errorf("Value = %q, want %q", r.Value, "0")
	}
	if r.Threshold != "30" {
		t.Errorf("Threshold = %q, want %q", r.Threshold, "30")
	}
}

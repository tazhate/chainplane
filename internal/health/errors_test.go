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
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrNoMetricData", ErrNoMetricData, "health: no metric data available"},
		{"ErrPrometheusUnhealthy", ErrPrometheusUnhealthy, "health: prometheus is not healthy"},
		{"ErrUnsupportedChain", ErrUnsupportedChain, "health: unsupported blockchain type"},
		{"ErrReplacementAnnotationMissing", ErrReplacementAnnotationMissing, "health: replacement annotation missing"},
		{"ErrReplacementTimeout", ErrReplacementTimeout, "health: replacement sync timeout exceeded"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestQueryError(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("connection refused")
	qe := &QueryError{Query: `up{job="prom"}`, Err: inner}

	t.Run("Error message format", func(t *testing.T) {
		t.Parallel()
		want := `prometheus query "up{job=\"prom\"}": connection refused`
		if got := qe.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Unwrap returns inner error", func(t *testing.T) {
		t.Parallel()
		if got := qe.Unwrap(); got != inner {
			t.Errorf("Unwrap() = %v, want %v", got, inner)
		}
	})

	t.Run("errors.Is matches inner", func(t *testing.T) {
		t.Parallel()
		wrapped := &QueryError{Query: "test", Err: ErrNoMetricData}
		if !errors.Is(wrapped, ErrNoMetricData) {
			t.Error("errors.Is should match inner ErrNoMetricData")
		}
	})

	t.Run("errors.As extracts QueryError", func(t *testing.T) {
		t.Parallel()
		wrapped := fmt.Errorf("outer: %w", qe)
		var target *QueryError
		if !errors.As(wrapped, &target) {
			t.Fatal("errors.As should find QueryError")
		}
		if target.Query != qe.Query {
			t.Errorf("Query = %q, want %q", target.Query, qe.Query)
		}
	})
}

func TestPhaseError(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("api timeout")
	pe := &PhaseError{Phase: ReplacementDraining, Err: inner}

	t.Run("Error message format", func(t *testing.T) {
		t.Parallel()
		want := "replacement phase Draining: api timeout"
		if got := pe.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Unwrap returns inner error", func(t *testing.T) {
		t.Parallel()
		if got := pe.Unwrap(); got != inner {
			t.Errorf("Unwrap() = %v, want %v", got, inner)
		}
	})

	t.Run("errors.As extracts PhaseError", func(t *testing.T) {
		t.Parallel()
		wrapped := fmt.Errorf("wrap: %w", pe)
		var target *PhaseError
		if !errors.As(wrapped, &target) {
			t.Fatal("errors.As should find PhaseError")
		}
		if target.Phase != ReplacementDraining {
			t.Errorf("Phase = %q, want %q", target.Phase, ReplacementDraining)
		}
	})

	t.Run("errors.Is matches wrapped sentinel", func(t *testing.T) {
		t.Parallel()
		pe2 := &PhaseError{Phase: ReplacementVerifying, Err: ErrReplacementTimeout}
		if !errors.Is(pe2, ErrReplacementTimeout) {
			t.Error("errors.Is should match wrapped ErrReplacementTimeout")
		}
	})
}

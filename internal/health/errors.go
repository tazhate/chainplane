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
// Package health implements node-health evaluation and automated pod
// replacement for blockchain nodes managed by the operator.
//
// Five trigger conditions (sync lag, error rate, latency, crash loop, disk
// usage) are evaluated against Prometheus metrics. When any trigger fires,
// the replacement manager orchestrates a blue/green pod swap.
package health

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by the health subsystem.
var (
	// ErrNoMetricData indicates that Prometheus returned an empty result set
	// for the requested query.
	ErrNoMetricData = errors.New("health: no metric data available")

	// ErrPrometheusUnhealthy indicates that the Prometheus instance did not
	// pass its readiness check.
	ErrPrometheusUnhealthy = errors.New("health: prometheus is not healthy")

	// ErrUnsupportedChain indicates that the requested blockchain type has no
	// configured metric mapping.
	ErrUnsupportedChain = errors.New("health: unsupported blockchain type")

	// ErrReplacementAnnotationMissing indicates that a required replacement
	// workflow annotation was not found on the ChainInstance resource.
	ErrReplacementAnnotationMissing = errors.New("health: replacement annotation missing")

	// ErrReplacementTimeout indicates that the replacement pod did not become
	// healthy within the configured sync timeout.
	ErrReplacementTimeout = errors.New("health: replacement sync timeout exceeded")
)

// QueryError wraps a Prometheus query failure with the PromQL expression that
// caused it.
type QueryError struct {
	Query string
	Err   error
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("prometheus query %q: %v", e.Query, e.Err)
}

func (e *QueryError) Unwrap() error { return e.Err }

// PhaseError wraps an error that occurred during a specific replacement phase.
type PhaseError struct {
	Phase ReplacementPhase
	Err   error
}

func (e *PhaseError) Error() string {
	return fmt.Sprintf("replacement phase %s: %v", e.Phase, e.Err)
}

func (e *PhaseError) Unwrap() error { return e.Err }

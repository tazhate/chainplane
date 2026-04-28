package health

import (
	"fmt"
	"strconv"
	"strings"
)

// TriggerName identifies one of the health evaluation triggers.
type TriggerName string

const (
	TriggerSyncLag   TriggerName = "sync_lag"
	TriggerErrorRate TriggerName = "error_rate"
	TriggerLatency   TriggerName = "latency"
	TriggerCrashLoop TriggerName = "crash_loop"
	TriggerDiskUsage TriggerName = "disk_usage"
)

// allTriggers defines the canonical evaluation order.
var allTriggers = []TriggerName{
	TriggerSyncLag,
	TriggerErrorRate,
	TriggerLatency,
	TriggerCrashLoop,
	TriggerDiskUsage,
}

// AllTriggers returns the canonical list of trigger names in evaluation order.
// The returned slice is a copy safe to modify.
func AllTriggers() []TriggerName {
	out := make([]TriggerName, len(allTriggers))
	copy(out, allTriggers)
	return out
}

// TriggerResult holds the outcome of a single trigger evaluation.
type TriggerResult struct {
	// Fired is true when the health condition is violated.
	Fired bool
	// Value is the observed metric value formatted as a human-readable string.
	Value string
	// Threshold is the configured threshold formatted as a human-readable string.
	Threshold string
}

// HealthStatus aggregates trigger results for one pod.
//
//nolint:revive
type HealthStatus struct {
	// PodName is the Kubernetes pod that was evaluated.
	PodName string
	// Blockchain is the chain type of the pod.
	Blockchain BlockchainType
	// Results maps each trigger name to its evaluation result.
	Results map[TriggerName]TriggerResult
}

// Healthy returns true when no trigger has fired.
func (s HealthStatus) Healthy() bool {
	for _, r := range s.Results {
		if r.Fired {
			return false
		}
	}
	return true
}

// TriggeredNames returns the names of all triggers that fired.
func (s HealthStatus) TriggeredNames() []TriggerName {
	var names []TriggerName
	for name, r := range s.Results {
		if r.Fired {
			names = append(names, name)
		}
	}
	return names
}

// Summary returns a human-readable multi-line summary of trigger results.
func (s HealthStatus) Summary() string {
	var b strings.Builder
	b.WriteString("Health Trigger Summary:\n")
	b.WriteString("--------------------------------------------------\n")

	for _, name := range allTriggers {
		r, ok := s.Results[name]
		if !ok {
			continue
		}
		status := "OK"
		if r.Fired {
			status = "TRIGGERED"
		}
		fmt.Fprintf(&b, "%-15s %-15s (value: %s, threshold: %s)\n",
			name, status, r.Value, r.Threshold)
	}

	b.WriteString("--------------------------------------------------\n")
	fmt.Fprintf(&b, "Total triggers active: %d/%d\n",
		len(s.TriggeredNames()), len(allTriggers))
	return b.String()
}

// Thresholds holds all configurable trigger thresholds.
// Zero-value fields are replaced with production defaults when passed to
// NewEvaluator.
type Thresholds struct {
	// SyncLagETH is the max allowed block lag for Ethereum nodes (default 30).
	SyncLagETH float64
	// SyncLagSOL is the max allowed slot lag for Solana nodes (default 150).
	SyncLagSOL float64
	// SyncLagDuration is how long (minutes) lag must persist (default 10).
	SyncLagDuration int

	// ErrorRate is the max allowed error ratio 0..1 (default 0.05).
	ErrorRate float64
	// ErrorRateWindow is the PromQL rate window in minutes (default 5).
	ErrorRateWindow int

	// LatencyETH is the max p99 latency in seconds for Ethereum (default 2.0).
	LatencyETH float64
	// LatencySOL is the max p99 latency in seconds for Solana (default 0.5).
	LatencySOL float64
	// LatencyDuration is how long (minutes) latency must persist (default 5).
	LatencyDuration int

	// CrashLoopRestarts is the restart count that triggers (default 3).
	CrashLoopRestarts int
	// CrashLoopWindow is the observation window in minutes (default 10).
	CrashLoopWindow int

	// DiskUsage is the max allowed disk usage ratio 0..1 (default 0.9).
	DiskUsage float64
}

// DefaultThresholds returns production-ready defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		SyncLagETH:      30,
		SyncLagSOL:      150,
		SyncLagDuration: 10,

		ErrorRate:       0.05,
		ErrorRateWindow: 5,

		LatencyETH:      2.0,
		LatencySOL:      0.5,
		LatencyDuration: 5,

		CrashLoopRestarts: 3,
		CrashLoopWindow:   10,

		DiskUsage: 0.9,
	}
}

// applyDefaults returns a copy where zero-value fields are replaced by defaults.
func (t Thresholds) applyDefaults() Thresholds {
	d := DefaultThresholds()
	fields := []struct {
		dst *float64
		def float64
	}{
		{&t.SyncLagETH, d.SyncLagETH},
		{&t.SyncLagSOL, d.SyncLagSOL},
		{&t.ErrorRate, d.ErrorRate},
		{&t.LatencyETH, d.LatencyETH},
		{&t.LatencySOL, d.LatencySOL},
		{&t.DiskUsage, d.DiskUsage},
	}
	for _, f := range fields {
		if *f.dst == 0 {
			*f.dst = f.def
		}
	}
	intFields := []struct {
		dst *int
		def int
	}{
		{&t.SyncLagDuration, d.SyncLagDuration},
		{&t.ErrorRateWindow, d.ErrorRateWindow},
		{&t.LatencyDuration, d.LatencyDuration},
		{&t.CrashLoopRestarts, d.CrashLoopRestarts},
		{&t.CrashLoopWindow, d.CrashLoopWindow},
	}
	for _, f := range intFields {
		if *f.dst == 0 {
			*f.dst = f.def
		}
	}
	return t
}

// syncLagThreshold returns the sync lag threshold for the given chain.
func (t Thresholds) syncLagThreshold(chain BlockchainType) float64 {
	m := map[BlockchainType]float64{
		BlockchainEthereum: t.SyncLagETH,
		BlockchainSolana:   t.SyncLagSOL,
	}
	if v, ok := m[chain]; ok {
		return v
	}
	return t.SyncLagETH
}

// latencyThreshold returns the p99 latency threshold for the given chain.
func (t Thresholds) latencyThreshold(chain BlockchainType) float64 {
	m := map[BlockchainType]float64{
		BlockchainEthereum: t.LatencyETH,
		BlockchainSolana:   t.LatencySOL,
	}
	if v, ok := m[chain]; ok {
		return v
	}
	return t.LatencyETH
}

// formatMetric formats a float64 to a string with up to 4 decimal places,
// trimming trailing zeros for readability.
func formatMetric(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// noDataResult returns a safe TriggerResult representing missing metric data.
func noDataResult(threshold string) TriggerResult {
	return TriggerResult{Value: "0", Threshold: threshold}
}

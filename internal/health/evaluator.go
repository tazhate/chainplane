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
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Checker evaluates health triggers for blockchain pods using metrics
// from a MetricQuerier backend.
type Checker struct {
	m MetricQuerier
	t Thresholds
}

// NewChecker creates a Checker with the given metric querier and thresholds.
// Zero-value threshold fields are replaced by DefaultThresholds.
func NewChecker(m MetricQuerier, t Thresholds) *Checker {
	return &Checker{
		m: m,
		t: t.applyDefaults(),
	}
}

// triggerEval pairs a trigger name with its evaluation function.
type triggerEval struct {
	name TriggerName
	fn   func(ctx context.Context, pod string, chain BlockchainType) TriggerResult
}

// CheckAll evaluates every trigger for the given pod and returns the
// aggregated HealthStatus. If the metric backend is unreachable, all
// triggers report as not-fired (safe fallback).
func (c *Checker) CheckAll(ctx context.Context, pod string, chain BlockchainType) HealthStatus {
	s := HealthStatus{
		PodName:    pod,
		Blockchain: chain,
		Results:    make(map[TriggerName]TriggerResult, len(allTriggers)),
	}

	if !c.m.Healthy(ctx) {
		slog.Error("metric backend unreachable, skipping trigger evaluation")
		for _, name := range allTriggers {
			s.Results[name] = noDataResult("n/a")
		}
		return s
	}

	slog.Info("evaluating triggers", "pod", pod, "chain", string(chain))

	evals := []triggerEval{
		{TriggerSyncLag, c.evalSyncLag},
		{TriggerErrorRate, c.evalErrorRate},
		{TriggerLatency, c.evalLatency},
		{TriggerCrashLoop, c.evalCrashLoop},
		{TriggerDiskUsage, c.evalDiskUsage},
	}

	for _, ev := range evals {
		s.Results[ev.name] = c.safeEval(ctx, ev, pod, chain)
	}

	if fired := s.TriggeredNames(); len(fired) > 0 {
		names := make([]string, len(fired))
		for i, n := range fired {
			names[i] = string(n)
		}
		slog.Warn("pod has active triggers",
			"pod", pod, "count", len(fired), "triggers", strings.Join(names, ", "))
	} else {
		slog.Info("pod healthy, no triggers fired", "pod", pod)
	}

	return s
}

// safeEval wraps a single trigger evaluation with panic recovery.
func (c *Checker) safeEval(ctx context.Context, ev triggerEval, pod string, chain BlockchainType) (result TriggerResult) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("trigger evaluation panicked",
				"trigger", ev.name, "error", r)
			result = noDataResult("n/a")
		}
	}()
	return ev.fn(ctx, pod, chain)
}

// evalSyncLag checks whether the blockchain sync lag has exceeded the
// configured threshold for a sustained period.
func (c *Checker) evalSyncLag(ctx context.Context, pod string, chain BlockchainType) TriggerResult {
	cfg, ok := chainConfigs[chain]
	if !ok {
		slog.Error("no metric config for chain", "chain", chain)
		return noDataResult("n/a")
	}

	query := fmt.Sprintf(cfg.syncLagQuery, pod)
	threshold := c.t.syncLagThreshold(chain)

	lag, err := c.m.InstantValue(ctx, query)
	if err != nil {
		if errors.Is(err, ErrNoMetricData) {
			slog.Warn("no sync lag data", "pod", pod)
		} else {
			slog.Error("sync lag query failed", "pod", pod, "error", err)
		}
		return noDataResult(formatMetric(threshold))
	}

	slog.Debug("sync lag", "pod", pod, "lag", lag, "threshold", threshold)

	fired, sustainErr := c.m.Sustained(ctx, query, threshold, c.t.SyncLagDuration)
	if sustainErr != nil {
		slog.Debug("sustained check inconclusive", "pod", pod, "error", sustainErr)
	}

	if fired {
		slog.Warn("sync lag trigger fired",
			"pod", pod, "lag", lag, "threshold", threshold,
			"sustained_min", c.t.SyncLagDuration)
	}

	return TriggerResult{
		Fired:     fired,
		Value:     formatMetric(lag),
		Threshold: formatMetric(threshold),
	}
}

// evalErrorRate checks whether the RPC error rate exceeds the threshold.
func (c *Checker) evalErrorRate(ctx context.Context, pod string, _ BlockchainType) TriggerResult {
	window := c.t.ErrorRateWindow
	query := fmt.Sprintf(
		`sum(rate(eth_rpc_errors_total{pod=~"%s"}[%dm])) / `+
			`sum(rate(eth_rpc_requests_total{pod=~"%s"}[%dm]))`,
		pod, window, pod, window,
	)

	rate, err := c.m.InstantValue(ctx, query)
	if err != nil {
		if errors.Is(err, ErrNoMetricData) {
			slog.Warn("no error rate data", "pod", pod)
		} else {
			slog.Error("error rate query failed", "pod", pod, "error", err)
		}
		return noDataResult(formatMetric(c.t.ErrorRate))
	}

	slog.Debug("error rate", "pod", pod, "rate", rate, "threshold", c.t.ErrorRate)

	fired := rate > c.t.ErrorRate
	if fired {
		slog.Warn("error rate trigger fired",
			"pod", pod, "rate", rate, "threshold", c.t.ErrorRate)
	}

	return TriggerResult{
		Fired:     fired,
		Value:     formatMetric(rate),
		Threshold: formatMetric(c.t.ErrorRate),
	}
}

// evalLatency checks whether p99 RPC latency exceeds the threshold for a
// sustained period.
func (c *Checker) evalLatency(ctx context.Context, pod string, chain BlockchainType) TriggerResult {
	threshold := c.t.latencyThreshold(chain)
	query := fmt.Sprintf(
		`histogram_quantile(0.99, sum(rate(rpc_request_duration_seconds_bucket{pod=~"%s"}[5m])) by (le))`,
		pod,
	)

	latency, err := c.m.InstantValue(ctx, query)
	if err != nil {
		if errors.Is(err, ErrNoMetricData) {
			slog.Warn("no latency data", "pod", pod)
		} else {
			slog.Error("latency query failed", "pod", pod, "error", err)
		}
		return noDataResult(formatMetric(threshold))
	}

	slog.Debug("p99 latency", "pod", pod, "latency_s", latency, "threshold_s", threshold)

	fired, sustainErr := c.m.Sustained(ctx, query, threshold, c.t.LatencyDuration)
	if sustainErr != nil {
		slog.Debug("sustained check inconclusive", "pod", pod, "error", sustainErr)
	}

	if fired {
		slog.Warn("latency trigger fired",
			"pod", pod, "latency_s", latency, "threshold_s", threshold,
			"sustained_min", c.t.LatencyDuration)
	}

	return TriggerResult{
		Fired:     fired,
		Value:     formatMetric(latency),
		Threshold: formatMetric(threshold),
	}
}

// evalCrashLoop checks whether the pod has had excessive restarts within
// the configured time window.
func (c *Checker) evalCrashLoop(ctx context.Context, pod string, _ BlockchainType) TriggerResult {
	query := fmt.Sprintf(
		`kube_pod_container_status_restarts_total{pod=~"%s"}`, pod,
	)

	current, err := c.m.InstantValue(ctx, query)
	if err != nil {
		if errors.Is(err, ErrNoMetricData) {
			slog.Warn("no restart count data", "pod", pod)
		} else {
			slog.Error("restart count query failed", "pod", pod, "error", err)
		}
		return noDataResult(strconv.Itoa(c.t.CrashLoopRestarts))
	}

	// Retrieve the restart count at the start of the observation window.
	end := time.Now().UTC()
	start := end.Add(-time.Duration(c.t.CrashLoopWindow) * time.Minute)

	var pastRestarts float64
	series, rangeErr := c.m.RangeValues(ctx, query, start, start.Add(30*time.Second), "30s")
	if rangeErr == nil && len(series) > 0 && len(series[0].Values) > 0 {
		var valStr string
		if err := json.Unmarshal(series[0].Values[0][1], &valStr); err == nil {
			if v, parseErr := strconv.ParseFloat(valStr, 64); parseErr == nil {
				pastRestarts = v
			}
		}
	}

	restartsInWindow := current - pastRestarts

	slog.Debug("restarts in window",
		"pod", pod, "restarts", restartsInWindow,
		"window_min", c.t.CrashLoopWindow,
		"threshold", c.t.CrashLoopRestarts)

	fired := restartsInWindow >= float64(c.t.CrashLoopRestarts)
	if fired {
		slog.Warn("crash loop trigger fired",
			"pod", pod, "restarts", restartsInWindow,
			"window_min", c.t.CrashLoopWindow,
			"threshold", c.t.CrashLoopRestarts)
	}

	return TriggerResult{
		Fired:     fired,
		Value:     formatMetric(restartsInWindow),
		Threshold: strconv.Itoa(c.t.CrashLoopRestarts),
	}
}

// evalDiskUsage checks whether the container filesystem usage ratio exceeds
// the configured threshold.
func (c *Checker) evalDiskUsage(ctx context.Context, pod string, _ BlockchainType) TriggerResult {
	query := fmt.Sprintf(
		`sum(container_fs_usage_bytes{pod=~"%s"}) / `+
			`sum(container_fs_limit_bytes{pod=~"%s"})`,
		pod, pod,
	)

	usage, err := c.m.InstantValue(ctx, query)
	if err != nil {
		if errors.Is(err, ErrNoMetricData) {
			slog.Warn("no disk usage data", "pod", pod)
		} else {
			slog.Error("disk usage query failed", "pod", pod, "error", err)
		}
		return noDataResult(formatMetric(c.t.DiskUsage))
	}

	slog.Debug("disk usage", "pod", pod, "usage", usage, "threshold", c.t.DiskUsage)

	fired := usage > c.t.DiskUsage
	if fired {
		slog.Warn("disk usage trigger fired",
			"pod", pod, "usage", usage, "threshold", c.t.DiskUsage)
	}

	return TriggerResult{
		Fired:     fired,
		Value:     formatMetric(usage),
		Threshold: formatMetric(c.t.DiskUsage),
	}
}

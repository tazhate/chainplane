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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// maxResponseBytes limits the size of a Prometheus HTTP response body to 10 MiB.
const maxResponseBytes = 10 << 20

// MetricQuerier abstracts Prometheus metric access for testability.
// Implementations must be safe for concurrent use.
type MetricQuerier interface {
	// Healthy reports whether the metric backend is reachable.
	Healthy(ctx context.Context) bool
	// InstantValue executes an instant query expected to return a single
	// scalar/vector value.
	InstantValue(ctx context.Context, promql string) (float64, error)
	// Sustained returns true when the given metric has remained above the
	// threshold for at least the specified duration.
	Sustained(ctx context.Context, promql string, threshold float64, minutes int) (bool, error)
	// RangeValues executes a range query and returns the parsed matrix results.
	RangeValues(ctx context.Context, promql string, start, end time.Time, step string) ([]MatrixSeries, error)
}

// MatrixSeries is a single time series from a range query result.
type MatrixSeries struct {
	Metric map[string]string
	Values [][2]json.RawMessage // [[timestamp, "value"], ...]
}

// --- Prometheus HTTP API implementation ---

// prometheusEnvelope is the top-level envelope returned by the Prometheus API.
type prometheusEnvelope struct {
	Status string          `json:"status"`
	Error  string          `json:"error,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// queryPayload is the "data" portion of an instant or range query response.
type queryPayload struct {
	ResultType string            `json:"resultType"`
	Result     []json.RawMessage `json:"result"`
}

// vectorSample is a single instant-query vector element.
type vectorSample struct {
	Metric map[string]string  `json:"metric"`
	Value  [2]json.RawMessage `json:"value"`
}

// PrometheusClient queries the Prometheus HTTP API with retry and backoff.
type PrometheusClient struct {
	baseURL    string
	http       *http.Client
	maxRetries int
	backoff    time.Duration
}

// PrometheusOption configures a PrometheusClient via functional options.
type PrometheusOption func(*PrometheusClient)

// WithTimeout sets the per-request HTTP timeout (default 10s).
func WithTimeout(d time.Duration) PrometheusOption {
	return func(p *PrometheusClient) { p.http.Timeout = d }
}

// WithMaxRetries sets the maximum number of retry attempts (default 3).
func WithMaxRetries(n int) PrometheusOption {
	return func(p *PrometheusClient) { p.maxRetries = n }
}

// WithBackoffFactor sets the base duration for exponential backoff (default 500ms).
func WithBackoffFactor(d time.Duration) PrometheusOption {
	return func(p *PrometheusClient) { p.backoff = d }
}

// NewPrometheusClient creates a MetricQuerier backed by the Prometheus HTTP API.
// The baseURL should not include a trailing slash (e.g. "http://prometheus:9090").
func NewPrometheusClient(baseURL string, opts ...PrometheusOption) *PrometheusClient {
	p := &PrometheusClient{
		baseURL:    baseURL,
		http:       &http.Client{Timeout: 10 * time.Second},
		maxRetries: 3,
		backoff:    500 * time.Millisecond,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// compile-time interface check.
var _ MetricQuerier = (*PrometheusClient)(nil)

// Healthy checks whether the Prometheus server is reachable via its /-/healthy
// endpoint.
func (p *PrometheusClient) Healthy(ctx context.Context) bool {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.baseURL+"/-/healthy", nil)
	if err != nil {
		return false
	}
	resp, err := p.http.Do(req)
	if err != nil {
		slog.Error("prometheus readiness probe failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse
	return resp.StatusCode == http.StatusOK
}

// InstantValue executes an instant query expected to return a single scalar or
// vector value. Returns ErrNoMetricData when the result set is empty.
func (p *PrometheusClient) InstantValue(ctx context.Context, promql string) (float64, error) {
	data, err := p.instantQuery(ctx, promql)
	if err != nil {
		return 0, &QueryError{Query: promql, Err: err}
	}

	var payload queryPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, &QueryError{Query: promql, Err: fmt.Errorf("decode payload: %w", err)}
	}

	if len(payload.Result) == 0 {
		return 0, &QueryError{Query: promql, Err: ErrNoMetricData}
	}
	if len(payload.Result) > 1 {
		slog.Warn("instant query returned multiple series, using first",
			"query", promql, "count", len(payload.Result))
	}

	var sample vectorSample
	if err := json.Unmarshal(payload.Result[0], &sample); err != nil {
		return 0, &QueryError{Query: promql, Err: fmt.Errorf("decode vector sample: %w", err)}
	}

	var valStr string
	if err := json.Unmarshal(sample.Value[1], &valStr); err != nil {
		return 0, &QueryError{Query: promql, Err: fmt.Errorf("decode value string: %w", err)}
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, &QueryError{Query: promql, Err: fmt.Errorf("parse float %q: %w", valStr, err)}
	}
	return val, nil
}

// Sustained returns true when the metric has been above the threshold for at
// least the specified duration. It uses 30-second steps and requires 80% of
// expected data points to be present.
func (p *PrometheusClient) Sustained(ctx context.Context, promql string, threshold float64, minutes int) (bool, error) {
	expr := fmt.Sprintf("%s > %g", promql, threshold)

	end := time.Now().UTC()
	start := end.Add(-time.Duration(minutes) * time.Minute)

	data, err := p.rangeQuery(ctx, expr, start, end, "30s")
	if err != nil {
		return false, &QueryError{Query: expr, Err: err}
	}

	var payload queryPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return false, &QueryError{Query: expr, Err: fmt.Errorf("decode range payload: %w", err)}
	}

	if len(payload.Result) == 0 {
		return false, nil
	}

	var series MatrixSeries
	if err := json.Unmarshal(payload.Result[0], &series); err != nil {
		return false, &QueryError{Query: expr, Err: fmt.Errorf("decode matrix series: %w", err)}
	}

	expectedPoints := float64(minutes*60) / 30.0
	sustained := float64(len(series.Values)) >= expectedPoints*0.8

	if sustained {
		slog.Info("metric sustained above threshold",
			"query", promql, "threshold", threshold, "duration_min", minutes)
	}
	return sustained, nil
}

// RangeValues executes a range query and returns parsed matrix series.
func (p *PrometheusClient) RangeValues(ctx context.Context, promql string, start, end time.Time, step string) ([]MatrixSeries, error) {
	data, err := p.rangeQuery(ctx, promql, start, end, step)
	if err != nil {
		return nil, &QueryError{Query: promql, Err: err}
	}

	var payload queryPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, &QueryError{Query: promql, Err: fmt.Errorf("decode range payload: %w", err)}
	}

	series := make([]MatrixSeries, 0, len(payload.Result))
	for _, raw := range payload.Result {
		var s MatrixSeries
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, &QueryError{Query: promql, Err: fmt.Errorf("decode matrix series: %w", err)}
		}
		series = append(series, s)
	}
	return series, nil
}

// --- HTTP transport helpers ---

// instantQuery performs a GET against /api/v1/query.
func (p *PrometheusClient) instantQuery(ctx context.Context, promql string) (json.RawMessage, error) {
	u, err := url.Parse(p.baseURL + "/api/v1/query")
	if err != nil {
		return nil, fmt.Errorf("parse endpoint URL: %w", err)
	}
	u.RawQuery = url.Values{"query": {promql}}.Encode()
	return p.get(ctx, u.String())
}

// rangeQuery performs a GET against /api/v1/query_range.
func (p *PrometheusClient) rangeQuery(ctx context.Context, promql string, start, end time.Time, step string) (json.RawMessage, error) {
	u, err := url.Parse(p.baseURL + "/api/v1/query_range")
	if err != nil {
		return nil, fmt.Errorf("parse endpoint URL: %w", err)
	}
	u.RawQuery = url.Values{
		"query": {promql},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {step},
	}.Encode()
	return p.get(ctx, u.String())
}

// get executes an HTTP GET with exponential backoff retry and returns the
// "data" field from the Prometheus response envelope.
func (p *PrometheusClient) get(ctx context.Context, rawURL string) (json.RawMessage, error) {
	var lastErr error
	for attempt := range p.maxRetries {
		if attempt > 0 {
			wait := p.backoff * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build HTTP request: %w", err)
		}

		resp, err := p.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP GET: %w", err)
			slog.Warn("prometheus request failed, retrying",
				"attempt", attempt+1, "error", err)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response body: %w", readErr)
			continue
		}

		// Retry on server errors and rate limiting.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
			slog.Warn("prometheus returned retryable status",
				"status", resp.StatusCode, "attempt", attempt+1)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
		}

		var envelope prometheusEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("decode prometheus envelope: %w", err)
		}

		if envelope.Status != "success" {
			return nil, fmt.Errorf("prometheus API error: %s", envelope.Error)
		}

		return envelope.Data, nil
	}

	return nil, fmt.Errorf("all %d retries exhausted: %w", p.maxRetries, lastErr)
}

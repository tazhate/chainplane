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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// promResponse builds a valid Prometheus API JSON envelope.
func promResponse(resultType string, results ...json.RawMessage) []byte {
	payload := queryPayload{ResultType: resultType, Result: results}
	data, _ := json.Marshal(payload)
	env := prometheusEnvelope{Status: "success", Data: data}
	b, _ := json.Marshal(env)
	return b
}

// vectorResult builds a single vector sample JSON.
func vectorResult(metric map[string]string, ts float64, value string) json.RawMessage {
	tsBytes, _ := json.Marshal(ts)
	valBytes, _ := json.Marshal(value)
	s := vectorSample{
		Metric: metric,
		Value:  [2]json.RawMessage{tsBytes, valBytes},
	}
	b, _ := json.Marshal(s)
	return b
}

// matrixResult builds a single matrix series JSON with given values.
func matrixResult(metric map[string]string, values [][2]interface{}) json.RawMessage {
	var rawValues [][2]json.RawMessage
	for _, v := range values {
		ts, _ := json.Marshal(v[0])
		val, _ := json.Marshal(v[1])
		rawValues = append(rawValues, [2]json.RawMessage{ts, val})
	}
	s := MatrixSeries{Metric: metric, Values: rawValues}
	b, _ := json.Marshal(s)
	return b
}

func TestPrometheusClient_Healthy(t *testing.T) {
	t.Parallel()

	t.Run("healthy endpoint returns 200", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/-/healthy" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		if !c.Healthy(context.Background()) {
			t.Error("expected Healthy() = true")
		}
	})

	t.Run("unhealthy endpoint returns 503", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		if c.Healthy(context.Background()) {
			t.Error("expected Healthy() = false for 503")
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		t.Parallel()
		c := NewPrometheusClient("http://127.0.0.1:1", WithTimeout(100*time.Millisecond))
		if c.Healthy(context.Background()) {
			t.Error("expected Healthy() = false for unreachable server")
		}
	})
}

func TestPrometheusClient_InstantValue(t *testing.T) {
	t.Parallel()

	t.Run("valid single vector result", func(t *testing.T) {
		t.Parallel()
		body := promResponse("vector",
			vectorResult(map[string]string{"__name__": "up"}, 1234567890, "42.5"),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/v1/query") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		val, err := c.InstantValue(context.Background(), "up")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 42.5 {
			t.Errorf("InstantValue = %v, want 42.5", val)
		}
	})

	t.Run("empty result returns ErrNoMetricData", func(t *testing.T) {
		t.Parallel()
		body := promResponse("vector") // no results
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		_, err := c.InstantValue(context.Background(), "missing_metric")
		if err == nil {
			t.Fatal("expected error for empty result")
		}
		if !errors.Is(err, ErrNoMetricData) {
			t.Errorf("expected ErrNoMetricData, got: %v", err)
		}
		var qe *QueryError
		if !errors.As(err, &qe) {
			t.Error("expected error to be wrapped in QueryError")
		}
	})

	t.Run("invalid value string returns error", func(t *testing.T) {
		t.Parallel()
		body := promResponse("vector",
			vectorResult(nil, 1234567890, "not_a_number"),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		_, err := c.InstantValue(context.Background(), "bad_metric")
		if err == nil {
			t.Fatal("expected error for non-numeric value")
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"success","data":"not_a_payload"}`))
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		_, err := c.InstantValue(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error for malformed payload")
		}
	})
}

func TestPrometheusClient_Sustained(t *testing.T) {
	t.Parallel()

	t.Run("sustained above threshold with enough points", func(t *testing.T) {
		t.Parallel()
		// 10 minutes at 30s step = 20 expected points. 80% = 16 points needed.
		var values [][2]interface{}
		for i := range 20 {
			values = append(values, [2]interface{}{float64(1000 + i*30), "50"})
		}
		body := promResponse("matrix",
			matrixResult(map[string]string{"__name__": "lag"}, values),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		sustained, err := c.Sustained(context.Background(), "lag", 30, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sustained {
			t.Error("expected sustained=true with 20/20 points")
		}
	})

	t.Run("not sustained with too few points", func(t *testing.T) {
		t.Parallel()
		// Only 5 points for 10 min window (need 16).
		var values [][2]interface{}
		for i := range 5 {
			values = append(values, [2]interface{}{float64(1000 + i*30), "50"})
		}
		body := promResponse("matrix",
			matrixResult(map[string]string{"__name__": "lag"}, values),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		sustained, err := c.Sustained(context.Background(), "lag", 30, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sustained {
			t.Error("expected sustained=false with only 5 points")
		}
	})

	t.Run("empty result returns false", func(t *testing.T) {
		t.Parallel()
		body := promResponse("matrix") // no series
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		sustained, err := c.Sustained(context.Background(), "lag", 30, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sustained {
			t.Error("expected sustained=false for empty result")
		}
	})

	t.Run("exactly at 80% threshold", func(t *testing.T) {
		t.Parallel()
		// 5 minutes = 10 expected points. 80% = 8.
		var values [][2]interface{}
		for i := range 8 {
			values = append(values, [2]interface{}{float64(1000 + i*30), "50"})
		}
		body := promResponse("matrix",
			matrixResult(map[string]string{}, values),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		sustained, err := c.Sustained(context.Background(), "metric", 10, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sustained {
			t.Error("expected sustained=true at exactly 80% of points")
		}
	})
}

func TestPrometheusClient_RangeValues(t *testing.T) {
	t.Parallel()

	t.Run("parses multiple series", func(t *testing.T) {
		t.Parallel()
		body := promResponse("matrix",
			matrixResult(map[string]string{"pod": "a"}, [][2]interface{}{{1000.0, "1"}, {1030.0, "2"}}),
			matrixResult(map[string]string{"pod": "b"}, [][2]interface{}{{1000.0, "3"}}),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL)
		start := time.Unix(1000, 0)
		end := time.Unix(1060, 0)
		series, err := c.RangeValues(context.Background(), "metric", start, end, "30s")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(series) != 2 {
			t.Fatalf("got %d series, want 2", len(series))
		}
		if series[0].Metric["pod"] != "a" {
			t.Errorf("series[0].Metric[pod] = %q, want %q", series[0].Metric["pod"], "a")
		}
		if len(series[0].Values) != 2 {
			t.Errorf("series[0] has %d values, want 2", len(series[0].Values))
		}
	})
}

func TestPrometheusClient_RetryOn429And500(t *testing.T) {
	t.Parallel()

	t.Run("retries on 429 then succeeds", func(t *testing.T) {
		t.Parallel()
		var attempts atomic.Int32
		body := promResponse("vector",
			vectorResult(nil, 1000, "1"),
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := attempts.Add(1)
			if n <= 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("rate limited"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL, WithMaxRetries(3), WithBackoffFactor(1*time.Millisecond))
		val, err := c.InstantValue(context.Background(), "test")
		if err != nil {
			t.Fatalf("expected success after retries, got: %v", err)
		}
		if val != 1 {
			t.Errorf("value = %v, want 1", val)
		}
		if got := attempts.Load(); got != 3 {
			t.Errorf("attempts = %d, want 3", got)
		}
	})

	t.Run("retries on 500 then exhausts", func(t *testing.T) {
		t.Parallel()
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer srv.Close()

		c := NewPrometheusClient(srv.URL, WithMaxRetries(2), WithBackoffFactor(1*time.Millisecond))
		_, err := c.InstantValue(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		if !strings.Contains(err.Error(), "retries exhausted") {
			t.Errorf("error should mention retries exhausted: %v", err)
		}
		if got := attempts.Load(); got != 2 {
			t.Errorf("attempts = %d, want 2", got)
		}
	})
}

func TestPrometheusClient_Timeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewPrometheusClient(srv.URL,
		WithTimeout(50*time.Millisecond),
		WithMaxRetries(1),
		WithBackoffFactor(1*time.Millisecond),
	)
	_, err := c.InstantValue(context.Background(), "slow_metric")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestPrometheusClient_NonRetryableError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	c := NewPrometheusClient(srv.URL)
	_, err := c.InstantValue(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("error should mention HTTP 400: %v", err)
	}
}

func TestPrometheusClient_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		env := prometheusEnvelope{Status: "error", Error: "bad query syntax"}
		b, _ := json.Marshal(env)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}))
	defer srv.Close()

	c := NewPrometheusClient(srv.URL)
	_, err := c.InstantValue(context.Background(), "bad{")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "bad query syntax") {
		t.Errorf("error should contain API error message: %v", err)
	}
}

func TestPrometheusClient_Options(t *testing.T) {
	t.Parallel()

	c := NewPrometheusClient("http://localhost:9090",
		WithTimeout(5*time.Second),
		WithMaxRetries(5),
		WithBackoffFactor(200*time.Millisecond),
	)

	if c.http.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.http.Timeout)
	}
	if c.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", c.maxRetries)
	}
	if c.backoff != 200*time.Millisecond {
		t.Errorf("backoff = %v, want 200ms", c.backoff)
	}
}

func TestPrometheusClient_ContextCancelDuringRetry(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so backoff select picks up the cancellation.
	cancel()

	c := NewPrometheusClient(srv.URL,
		WithMaxRetries(5),
		WithBackoffFactor(10*time.Second), // long backoff so cancel triggers
	)
	_, err := c.InstantValue(ctx, "test")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

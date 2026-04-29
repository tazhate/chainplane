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
package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --------------------------------------------------------------------------
// Custom RPC error types
// --------------------------------------------------------------------------

// ErrRPCTimeout indicates the RPC call timed out (context deadline exceeded).
var ErrRPCTimeout = errors.New("rpc: request timed out")

// ErrRPCUnavailable indicates the RPC endpoint returned a 5xx or is unreachable.
var ErrRPCUnavailable = errors.New("rpc: service unavailable")

// RPCError wraps a JSON-RPC error response with code and message.
type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// --------------------------------------------------------------------------
// Shared HTTP client with connection pooling
// --------------------------------------------------------------------------

// rpcClient is a package-level HTTP client with keep-alive and connection pooling
// reused across all RPC calls to avoid per-request transport allocation.
var rpcClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

// maxResponseBytes is the upper bound for RPC response body reads (10 MiB).
// Prevents unbounded memory allocation from malicious or misbehaving endpoints.
const maxResponseBytes = 10 << 20

// --------------------------------------------------------------------------
// JSON-RPC protocol types
// --------------------------------------------------------------------------

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --------------------------------------------------------------------------
// Core RPC call functions
// --------------------------------------------------------------------------

// callRPC makes a JSON-RPC call and returns the raw result.
// Uses the shared pooled HTTP client with io.LimitReader for response bodies.
func callRPC(ctx context.Context, rpcURL, method string, params []any) (json.RawMessage, error) {
	reqBody, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rpcClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %w", ErrRPCTimeout, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrRPCUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read rpc response: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		// Include HTTP status and body prefix so callers can detect non-JSON responses
		// like "Work queue depth exceeded" (HTTP 503) or HTML error pages.
		prefix := string(body)
		if len(prefix) > 80 {
			prefix = prefix[:80]
		}
		return nil, fmt.Errorf("http %d: %s: %w", resp.StatusCode, strings.TrimSpace(prefix), err)
	}

	if rpcResp.Error != nil {
		return nil, &RPCError{Code: rpcResp.Error.Code, Message: rpcResp.Error.Message}
	}

	return rpcResp.Result, nil
}

// callRPCWithRetry calls callRPC with up to maxRetries on transient errors,
// using exponential backoff (2s, 4s, 8s).
func callRPCWithRetry(ctx context.Context, rpcURL, method string, params []any, maxRetries int) (json.RawMessage, error) {
	var lastErr error
	delay := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := callRPC(ctx, rpcURL, method, params)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isTransientRPCError(err) || attempt == maxRetries {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(delay):
		}
		delay *= 2
	}
	return nil, lastErr
}

// --------------------------------------------------------------------------
// Error classification helpers
// --------------------------------------------------------------------------

// isTransientRPCError returns true when the error is a transient condition that
// may resolve on retry (timeouts, work queue full, loading, etc.).
func isTransientRPCError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRPCTimeout) || errors.Is(err, ErrRPCUnavailable) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "work queue") ||
		strings.Contains(s, "503") ||
		strings.Contains(s, "500") ||
		strings.Contains(s, "service unavailable") ||
		strings.Contains(s, "-28") ||
		strings.Contains(s, "loading")
}

// isContextTimeout returns true when an HTTP error is caused by a context
// deadline/timeout rather than a hard network failure (e.g. connection refused).
// Used by adapters to distinguish "process busy" from "process down".
func isContextTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRPCTimeout) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "context canceled") ||
		strings.Contains(s, "deadline exceeded")
}

// --------------------------------------------------------------------------
// Numeric helpers
// --------------------------------------------------------------------------

// parseDecimalInt64 parses a decimal string to int64, returning 0 on error.
func parseDecimalInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// progressFromBlocks calculates sync progress percentage.
// Returns 0 if highest is 0 (unknown), 100 if current >= highest.
func progressFromBlocks(current, highest int64) float64 {
	if highest <= 0 || current <= 0 {
		return 0
	}
	if current >= highest {
		return 100.0
	}
	return float64(current) / float64(highest) * 100.0
}

// pseudoBlock returns a time-varying block number for stall-detector evasion
// during long init phases where real block height stays at zero.
func pseudoBlock() int64 {
	return time.Now().Unix() & 0x7FFFF // changes every second, max ~524287
}

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
	"testing"
)

func TestParseDecimalInt64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "positive decimal", input: "12345", want: 12345},
		{name: "zero", input: "0", want: 0},
		{name: "negative", input: "-100", want: -100},
		{name: "large number", input: "9999999999", want: 9999999999},
		{name: "empty string", input: "", want: 0},
		{name: "non-numeric", input: "abc", want: 0},
		{name: "hex prefix ignored", input: "0x1a", want: 0},
		{name: "leading zeros", input: "007", want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDecimalInt64(tt.input)
			if got != tt.want {
				t.Errorf("parseDecimalInt64(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestProgressFromBlocks(t *testing.T) {
	tests := []struct {
		name            string
		current         int64
		highest         int64
		wantZero        bool
		wantHundred     bool
		wantApproximate float64
	}{
		{name: "zero highest", current: 100, highest: 0, wantZero: true},
		{name: "zero current", current: 0, highest: 100, wantZero: true},
		{name: "both zero", current: 0, highest: 0, wantZero: true},
		{name: "negative highest", current: 50, highest: -1, wantZero: true},
		{name: "negative current", current: -1, highest: 100, wantZero: true},
		{name: "current equals highest", current: 1000, highest: 1000, wantHundred: true},
		{name: "current exceeds highest", current: 1500, highest: 1000, wantHundred: true},
		{name: "half synced", current: 500, highest: 1000, wantApproximate: 50.0},
		{name: "quarter synced", current: 250, highest: 1000, wantApproximate: 25.0},
		{name: "almost synced", current: 999, highest: 1000, wantApproximate: 99.9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := progressFromBlocks(tt.current, tt.highest)
			switch {
			case tt.wantZero:
				if got != 0 {
					t.Errorf("progressFromBlocks(%d, %d) = %f, want 0", tt.current, tt.highest, got)
				}
			case tt.wantHundred:
				if got != 100.0 {
					t.Errorf("progressFromBlocks(%d, %d) = %f, want 100.0", tt.current, tt.highest, got)
				}
			default:
				diff := got - tt.wantApproximate
				if diff < -0.1 || diff > 0.1 {
					t.Errorf("progressFromBlocks(%d, %d) = %f, want ~%f", tt.current, tt.highest, got, tt.wantApproximate)
				}
			}
		})
	}
}

func TestHexToInt64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "zero", input: "0x0", want: 0},
		{name: "small hex", input: "0xa", want: 10},
		{name: "block number", input: "0x1234", want: 4660},
		{name: "no prefix", input: "ff", want: 255},
		{name: "empty string", input: "", want: 0},
		{name: "just prefix", input: "0x", want: 0},
		{name: "uppercase", input: "0xABCD", want: 43981},
		{name: "large value", input: "0x5f5e100", want: 100000000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hexToInt64(tt.input)
			if got != tt.want {
				t.Errorf("hexToInt64(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsTransientRPCError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "deadline exceeded", err: errStr("context deadline exceeded"), want: true},
		{name: "work queue", err: errStr("Work queue depth exceeded"), want: true},
		{name: "503", err: errStr("http 503: service unavailable"), want: true},
		{name: "500", err: errStr("http 500: internal server error"), want: true},
		{name: "loading", err: errStr("loading block index"), want: true},
		{name: "bitcoin -28", err: errStr("rpc error -28: loading block index"), want: true},
		{name: "connection refused", err: errStr("connection refused"), want: false},
		{name: "random error", err: errStr("unexpected EOF"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientRPCError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientRPCError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsContextTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "deadline exceeded", err: errStr("context deadline exceeded"), want: true},
		{name: "context canceled", err: errStr("context canceled"), want: true},
		{name: "connection refused", err: errStr("connection refused"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContextTimeout(tt.err)
			if got != tt.want {
				t.Errorf("isContextTimeout(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// errStr is a simple error type for testing.
type errStr string

func (e errStr) Error() string { return string(e) }

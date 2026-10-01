/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"slices"
	"strings"
	"testing"
)

func TestPlatformsFromRaw(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		platforms []string
		isIndex   bool
	}{
		{
			name: "oci index with attestations",
			raw: `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[
				{"platform":{"architecture":"amd64","os":"linux"}},
				{"platform":{"architecture":"arm64","os":"linux","variant":"v8"}},
				{"platform":{"architecture":"unknown","os":"unknown"}}]}`,
			platforms: []string{"linux/amd64", "linux/arm64/v8"},
			isIndex:   true,
		},
		{
			name: "docker manifest list without amd64",
			raw: `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.list.v2+json",
				"manifests":[{"platform":{"architecture":"arm64","os":"linux"}}]}`,
			platforms: []string{"linux/arm64"},
			isIndex:   true,
		},
		{
			name: "single docker v2 manifest",
			raw: `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json",
				"config":{"digest":"sha256:x"}}`,
		},
		{
			name:      "schema 1",
			raw:       `{"schemaVersion":1,"architecture":"amd64"}`,
			platforms: []string{"linux/amd64"},
		},
	}
	for _, tt := range tests {
		platforms, isIndex, err := platformsFromRaw([]byte(tt.raw))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !slices.Equal(platforms, tt.platforms) || isIndex != tt.isIndex {
			t.Errorf("%s: got %q index=%v, want %q index=%v", tt.name, platforms, isIndex, tt.platforms, tt.isIndex)
		}
	}
	if _, _, err := platformsFromRaw([]byte("not json")); err == nil {
		t.Error("expected an error for invalid JSON")
	}
}

func TestRenderReport(t *testing.T) {
	results := []result{
		{chain: "ethereum", client: "geth", image: "geth:1", status: statusPass, detail: "index: linux/amd64"},
		{
			chain: "bitcoin", image: "bitcoind:1", level: 1, status: statusFail,
			detail: "exited | code 1\nnext", notes: []string{"init containers skipped: x"},
		},
		{chain: "dash", level: 1, status: statusSkip, detail: strings.Repeat("x", 300)},
	}
	out := renderReport(results)
	for _, want := range []string{
		"3 checked: 1 PASS, 0 WARN, 1 FAIL, 1 SKIP",
		"| bitcoin | (default) | bitcoind:1 | 1 | FAIL | exited \\| code 1 next [init containers skipped: x] |",
		"| ethereum | geth | geth:1 | 0 | PASS | index: linux/amd64 |",
		"…",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "| bitcoin") > strings.Index(out, "| dash") {
		t.Error("rows are not sorted by chain")
	}
	if !anyFailed(results) || anyFailed(results[:1]) {
		t.Error("anyFailed mismatch")
	}
}

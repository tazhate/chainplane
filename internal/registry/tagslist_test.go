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
package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/tazhate/chainplane/internal/adapters"
)

// tagsListServer serves tags in fixed-size pages with GHCR-style relative
// Link headers that carry n=0, like ghcr.io does.
func tagsListServer(t *testing.T, path string, tags []string, perPage int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
	})
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if n := r.URL.Query().Get("n"); n != fmt.Sprint(tagsListPageSize) {
			t.Errorf("expected n=%d on every page, got %q", tagsListPageSize, n)
		}
		start := 0
		if last := r.URL.Query().Get("last"); last != "" {
			start = slices.Index(tags, last) + 1
		}
		end := min(start+perPage, len(tags))
		if end < len(tags) {
			w.Header().Set("Link", fmt.Sprintf(`<%s?last=%s&n=0>; rel="next"`, path, tags[end-1]))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": tags[start:end]})
	})
	return httptest.NewServer(mux)
}

func numberedTags(n int) []string {
	tags := make([]string, 0, n)
	for i := range n {
		tags = append(tags, fmt.Sprintf("v1.%d.0", i))
	}
	return tags
}

func TestGHCRFollowsLinkPagination(t *testing.T) {
	tags := numberedTags(250)
	srv := tagsListServer(t, "/v2/cosmos/gaia/tags/list", tags, 100)
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &ghcrClient{http: &http.Client{
		Transport: &ociRewriteTransport{base: http.DefaultTransport, target: target},
		Timeout:   5 * time.Second,
	}}

	entries, err := c.LatestTags(t.Context(), adapters.ChainVersionPolicy{
		Repository: "cosmos/gaia",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}, 25)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	// All pages read, nothing truncated to maxResults: the newest tag lives on
	// the last page because GHCR lists oldest first.
	if len(entries) != len(tags) {
		t.Fatalf("expected %d tags, got %d", len(tags), len(entries))
	}
	if got := Newest(ociTagList(entries), ""); got != "v1.249.0" {
		t.Fatalf("newest = %q, want v1.249.0", got)
	}
}

func TestOCIStandardFollowsLinkPagination(t *testing.T) {
	tags := numberedTags(30)
	srv := tagsListServer(t, "/v2/acme/node/tags/list", tags, 7)
	defer srv.Close()

	c := pointTo(t, srv, "public.ecr.aws")
	entries, err := c.LatestTags(t.Context(), adapters.ChainVersionPolicy{
		Repository: "public.ecr.aws/acme/node",
		TagPattern: `.*`,
	}, 5)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if len(entries) != len(tags) {
		t.Fatalf("expected %d tags across pages, got %d", len(tags), len(entries))
	}
}

func TestOCIGARReturnsAllMatchingTags(t *testing.T) {
	// 40 digests in a JSON object: Go map iteration order is random, so any
	// cut before the semver pick returned a different "latest" on each run.
	manifest := make(map[string]any, 40)
	for i, tag := range numberedTags(40) {
		manifest[fmt.Sprintf("sha256:%064d", i)] = map[string]any{"tag": []string{tag}}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"manifest": manifest})
	}))
	defer srv.Close()

	c := pointTo(t, srv, garHost)
	for range 5 {
		entries, err := c.LatestTags(t.Context(), adapters.ChainVersionPolicy{
			Repository: "oplabs-tools-artifacts/images/op-geth",
			TagPattern: `^v\d+\.\d+\.\d+$`,
		}, 25)
		if err != nil {
			t.Fatalf("LatestTags: %v", err)
		}
		if got := Newest(ociTagList(entries), ""); got != "v1.39.0" {
			t.Fatalf("newest = %q, want v1.39.0", got)
		}
	}
}

func TestResolveNextLink(t *testing.T) {
	const current = "https://ghcr.io/v2/a/b/tags/list?n=1000"
	tests := []struct {
		name, header, want string
	}{
		{"relative with n=0", `</v2/a/b/tags/list?last=v1&n=0>; rel="next"`, "https://ghcr.io/v2/a/b/tags/list?last=v1&n=1000"},
		{"absolute", `<https://other.example/v2/x/tags/list?last=z&n=50>; rel="next"`, "https://other.example/v2/x/tags/list?last=z&n=50"},
		{"no next", `</v2/a/b/tags/list?last=v0>; rel="prev"`, ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveNextLink(current, tt.header); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

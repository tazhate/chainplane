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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/tazhate/chainplane/internal/adapters"
)

// rewriteTransport redirects every outbound request to the test server,
// regardless of the host in the URL. This lets the handler emit realistic
// absolute hub.docker.com "next" links while the client still reaches httptest.
type rewriteTransport struct {
	target *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	req.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func newDockerHubTestClient(t *testing.T, server *httptest.Server) *dockerHubClient {
	t.Helper()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return &dockerHubClient{
		http: &http.Client{Transport: &rewriteTransport{target: target}},
	}
}

func writeTagsPage(w http.ResponseWriter, next string, names ...string) {
	results := make([]map[string]any, 0, len(names))
	for _, n := range names {
		results = append(results, map[string]any{"name": n, "tag_last_pushed": ""})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"next":    next,
		"results": results,
	})
}

func tagSet(entries []TagEntry) map[string]bool {
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		set[e.Tag] = true
	}
	return set
}

func TestDockerHubLatestTagsPaginates(t *testing.T) {
	var hits int32
	mux := http.NewServeMux()

	// Page 1 -> page 2 -> page 3, chained via absolute hub.docker.com URLs.
	mux.HandleFunc("/v2/repositories/library/bsc/tags", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		writeTagsPage(w, "https://hub.docker.com/v2/repositories/library/bsc/tags/p2",
			"v1.3.0", "skipme", "v1.3.1")
	})
	mux.HandleFunc("/v2/repositories/library/bsc/tags/p2", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		writeTagsPage(w, "https://hub.docker.com/v2/repositories/library/bsc/tags/p3",
			"v1.5.0", "latest", "v1.5.2")
	})
	mux.HandleFunc("/v2/repositories/library/bsc/tags/p3", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		writeTagsPage(w, "", "v1.6.0", "v1.6.1")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newDockerHubTestClient(t, server)
	policy := adapters.ChainVersionPolicy{Repository: "bsc", TagPattern: `^v\d+\.\d+\.\d+$`}

	entries, err := client.LatestTags(context.Background(), policy, 2)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}

	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 page fetches, got %d", got)
	}

	got := tagSet(entries)
	want := []string{"v1.3.0", "v1.3.1", "v1.5.0", "v1.5.2", "v1.6.0", "v1.6.1"}
	if len(entries) != len(want) {
		t.Fatalf("expected %d matching tags, got %d: %v", len(want), len(entries), entries)
	}
	for _, tag := range want {
		if !got[tag] {
			t.Fatalf("missing tag %q in %v", tag, entries)
		}
	}
	if got["skipme"] || got["latest"] {
		t.Fatalf("non-matching tag leaked into results: %v", entries)
	}

	// The real latest release lives on the deepest page; semver selection over
	// the collected superset must surface it.
	newest := ""
	for _, e := range entries {
		if newest == "" || IsNewer(e.Tag, newest, "") {
			newest = e.Tag
		}
	}
	if newest != "v1.6.1" {
		t.Fatalf("expected newest v1.6.1, got %s", newest)
	}
}

func TestDockerHubLatestTagsRespectsMaxPages(t *testing.T) {
	var hits int32
	mux := http.NewServeMux()

	// An infinite "next" chain with only non-matching tags: nothing ever
	// satisfies the surplus target, so only the page cap can stop the walk.
	mux.HandleFunc("/v2/repositories/library/harmony/tags", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		writeTagsPage(w,
			fmt.Sprintf("https://hub.docker.com/v2/repositories/library/harmony/tags?page=%d", n+1),
			"nope")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newDockerHubTestClient(t, server)
	policy := adapters.ChainVersionPolicy{Repository: "harmony", TagPattern: `^v\d+\.\d+\.\d+$`}

	entries, err := client.LatestTags(context.Background(), policy, 5)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no matching tags, got %v", entries)
	}
	if got := atomic.LoadInt32(&hits); got != dockerHubMaxPages {
		t.Fatalf("expected exactly %d page fetches, got %d", dockerHubMaxPages, got)
	}
}

func TestDockerHubLatestTagsStopsAtSurplus(t *testing.T) {
	var hits int32
	mux := http.NewServeMux()

	// Every page yields 4 matching tags and links onward forever. With
	// maxResults=2 the surplus target is 8, so the walk must stop after
	// exactly 2 pages rather than crawling the whole (infinite) repo.
	mux.HandleFunc("/v2/repositories/library/klaytn/tags", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		base := int(n) * 10
		writeTagsPage(w,
			fmt.Sprintf("https://hub.docker.com/v2/repositories/library/klaytn/tags?page=%d", n+1),
			fmt.Sprintf("v%d.0.0", base+1),
			fmt.Sprintf("v%d.0.0", base+2),
			fmt.Sprintf("v%d.0.0", base+3),
			fmt.Sprintf("v%d.0.0", base+4),
		)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newDockerHubTestClient(t, server)
	policy := adapters.ChainVersionPolicy{Repository: "klaytn", TagPattern: `^v\d+\.\d+\.\d+$`}

	entries, err := client.LatestTags(context.Background(), policy, 2)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 page fetches (surplus target 8), got %d", got)
	}
	if len(entries) < 8 {
		t.Fatalf("expected at least 8 collected tags, got %d", len(entries))
	}
}

func TestDockerHubLatestTagsContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/repositories/library/klaytn/tags", func(w http.ResponseWriter, r *http.Request) {
		writeTagsPage(w, "https://hub.docker.com/v2/repositories/library/klaytn/tags/p2", "v1.0.0")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newDockerHubTestClient(t, server)
	policy := adapters.ChainVersionPolicy{Repository: "klaytn", TagPattern: `^v\d+\.\d+\.\d+$`}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.LatestTags(ctx, policy, 2); err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

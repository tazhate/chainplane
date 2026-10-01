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
	"time"

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
	tests := []struct {
		name  string
		auth  *dockerHubAuth
		pages int32
	}{
		// Docker Hub 403s anonymous offsets past 1000, so stop before that.
		{"anonymous", nil, dockerHubAnonymousMaxPages},
		{"authenticated", &dockerHubAuth{username: "bot", secret: "pat"}, dockerHubMaxPages},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			mux := http.NewServeMux()
			mux.HandleFunc("POST /v2/auth/token", func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
			})

			// An infinite "next" chain with only non-matching tags: nothing ever
			// satisfies the surplus target, so only the page cap can stop the walk.
			mux.HandleFunc("/v2/repositories/library/harmony/tags", func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				writeTagsPage(w,
					fmt.Sprintf("https://hub.docker.com/v2/repositories/library/harmony/tags?page=%d", n+1),
					"nope")
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			client := newDockerHubTestClient(t, server)
			client.auth = tt.auth
			policy := adapters.ChainVersionPolicy{Repository: "harmony", TagPattern: `^v\d+\.\d+\.\d+$`}

			entries, err := client.LatestTags(t.Context(), policy, 5)
			if err != nil {
				t.Fatalf("LatestTags: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("expected no matching tags, got %v", entries)
			}
			if got := hits.Load(); got != tt.pages {
				t.Fatalf("expected exactly %d page fetches, got %d", tt.pages, got)
			}
		})
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

func TestDockerHubRetriesAfterRateLimit(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/repositories/acme/node/tags", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeTagsPage(w, "", "v1.2.3")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := newDockerHubTestClient(t, server)
	entries, err := c.LatestTags(t.Context(), adapters.ChainVersionPolicy{
		Repository: "acme/node",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}, 5)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if !tagSet(entries)["v1.2.3"] {
		t.Fatalf("expected v1.2.3 after retry, got %v", entries)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
}

func TestDockerHubForbiddenWithoutRateLimitIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/repositories/acme/private/tags", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := newDockerHubTestClient(t, server)
	_, err := c.LatestTags(t.Context(), adapters.ChainVersionPolicy{
		Repository: "acme/private",
		TagPattern: `.*`,
	}, 5)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected a single request for a plain 403, got %d", got)
	}
}

func TestDockerHubAuthSendsBearerToken(t *testing.T) {
	var logins atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/auth/token", func(w http.ResponseWriter, r *http.Request) {
		logins.Add(1)
		var creds map[string]string
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil ||
			creds["identifier"] != "bot" || creds["secret"] != "pat" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt-123"})
	})
	mux.HandleFunc("/v2/repositories/acme/node/tags", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next := ""
		if r.URL.Query().Get("page") == "" {
			next = "https://hub.docker.com/v2/repositories/acme/node/tags?page=2"
		}
		writeTagsPage(w, next, "v1.0."+r.URL.Query().Get("page"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := newDockerHubTestClient(t, server)
	c.auth = &dockerHubAuth{username: "bot", secret: "pat"}
	entries, err := c.LatestTags(t.Context(), adapters.ChainVersionPolicy{
		Repository: "acme/node",
		TagPattern: `.*`,
	}, 5)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected tags from both pages, got %v", entries)
	}
	if got := logins.Load(); got != 1 {
		t.Fatalf("expected one token exchange across pages, got %d", got)
	}
}

func TestDockerHubRetryWait(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tests := []struct {
		name   string
		header http.Header
		want   time.Duration
	}{
		{"retry-after", http.Header{"Retry-After": {"7"}}, 7 * time.Second},
		{"reset", http.Header{"X-Ratelimit-Reset": {"1000030"}}, 30 * time.Second},
		{"reset in past", http.Header{"X-Ratelimit-Reset": {"999990"}}, time.Second},
		{"capped", http.Header{"Retry-After": {"3600"}}, dockerHubMaxWait},
		{"no headers", http.Header{}, dockerHubDefaultWait},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dockerHubRetryWait(tt.header, now); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

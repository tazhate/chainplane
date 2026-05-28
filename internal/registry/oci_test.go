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
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tazhate/chainplane/internal/adapters"
)

// pointTo rewrites the OCI client's host so it talks to the test server.
// It returns a client configured to dial the test server for HTTPS hostnames.
func pointTo(t *testing.T, srv *httptest.Server, host string) *ociClient {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	// Custom transport rewrites scheme+host to the test server.
	rt := &rewriteTransport{base: http.DefaultTransport, target: u}
	return &ociClient{
		host: host,
		http: &http.Client{Transport: rt, Timeout: 5 * time.Second},
	}
}

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	return r.base.RoundTrip(req)
}

func TestOCIClient_GAR_NoTokenAndFlattensManifest(t *testing.T) {
	var tokenHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/token"):
			tokenHit = true
			http.Error(w, "should not be called for GAR", http.StatusInternalServerError)
		case r.URL.Path == "/v2/oplabs-tools-artifacts/images/op-geth/tags/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"child": [],
				"manifest": {
					"sha256:aaa": {"tag": ["v1.101411.2"]},
					"sha256:bbb": {"tag": ["v1.101408.0", "v1.101408.0-rc1"]},
					"sha256:ccc": {"tag": []},
					"sha256:ddd": {"tag": ["nightly"]}
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := pointTo(t, srv, garHost)
	policy := adapters.ChainVersionPolicy{
		Registry:   garHost,
		Repository: "oplabs-tools-artifacts/images/op-geth",
		TagPattern: `^v\d`,
	}

	entries, err := c.LatestTags(context.Background(), policy, 10)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if tokenHit {
		t.Errorf("GAR client must not call /token endpoint")
	}

	got := tagSet(entries)
	want := []string{"v1.101408.0", "v1.101408.0-rc1", "v1.101411.2"}
	sort.Strings(got)
	if !equalStrings(got, want) {
		t.Errorf("got tags %v, want %v", got, want)
	}
}

func TestOCIClient_GAR_TrimsHostPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/oplabs-tools-artifacts/images/op-geth/tags/list" {
			t.Errorf("unexpected request path %q (host prefix not trimmed?)", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"manifest":{"sha256:x":{"tag":["v1.0.0"]}}}`))
	}))
	defer srv.Close()

	c := pointTo(t, srv, garHost)
	policy := adapters.ChainVersionPolicy{
		Registry:   garHost,
		Repository: "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth",
		TagPattern: `^v\d`,
	}
	if _, err := c.LatestTags(context.Background(), policy, 5); err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
}

func TestOCIClient_Standard_UsesBearerToken(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"abc123"}`))
		case strings.HasPrefix(r.URL.Path, "/v2/") && strings.HasSuffix(r.URL.Path, "/tags/list"):
			sawAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":["v3.6.2","v3.6.1","nightly","v3.6.3"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := pointTo(t, srv, "public.ecr.aws")
	policy := adapters.ChainVersionPolicy{
		Registry:   "public.ecr.aws",
		Repository: "i6b2w2n6/nitro-node",
		TagPattern: `^v\d`,
	}

	entries, err := c.LatestTags(context.Background(), policy, 10)
	if err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
	if sawAuth != "Bearer abc123" {
		t.Errorf("expected Authorization=Bearer abc123, got %q", sawAuth)
	}

	want := []string{"v3.6.2", "v3.6.1", "v3.6.3"}
	if !equalStrings(tagSet(entries), want) {
		t.Errorf("got %v, want %v", tagSet(entries), want)
	}
}

func TestOCIClient_Standard_AccessTokenField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			// ECR Public returns access_token, not token.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"ecr-token"}`))
		case strings.HasSuffix(r.URL.Path, "/tags/list"):
			if got := r.Header.Get("Authorization"); got != "Bearer ecr-token" {
				t.Errorf("expected Bearer ecr-token, got %q", got)
			}
			_, _ = w.Write([]byte(`{"tags":["v1.0.0"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := pointTo(t, srv, "public.ecr.aws")
	policy := adapters.ChainVersionPolicy{
		Registry:   "public.ecr.aws",
		Repository: "x/y",
		TagPattern: `^v\d`,
	}
	if _, err := c.LatestTags(context.Background(), policy, 5); err != nil {
		t.Fatalf("LatestTags: %v", err)
	}
}

func TestOCIClient_GAR_ReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := pointTo(t, srv, garHost)
	policy := adapters.ChainVersionPolicy{
		Registry:   garHost,
		Repository: "x/y",
		TagPattern: `.*`,
	}
	_, err := c.LatestTags(context.Background(), policy, 5)
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got %v", err)
	}
}

func tagSet(entries []TagEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Tag)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := map[string]int{}
	for _, s := range a {
		am[s]++
	}
	for _, s := range b {
		am[s]--
		if am[s] < 0 {
			return false
		}
	}
	return true
}

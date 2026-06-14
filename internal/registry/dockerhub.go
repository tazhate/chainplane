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
	"net/url"
	"regexp"
	"time"

	"github.com/tazhate/chainplane/internal/adapters"
)

const dockerHubAPIBase = "https://hub.docker.com/v2/repositories"

// dockerHubPageSize is the per-page tag count requested from Docker Hub.
const dockerHubPageSize = 100

// dockerHubMaxPages caps how many pages we walk. Repositories such as harmony,
// bsc and klaytn carry thousands of tags; without a cap a pathological repo
// could send us into a near-unbounded crawl.
const dockerHubMaxPages = 20

type dockerHubClient struct {
	http *http.Client
}

func (c *dockerHubClient) httpClient() *http.Client {
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	return c.http
}

type dockerHubTagsResponse struct {
	// Next is the absolute URL of the following page, or "" on the last page.
	Next    string         `json:"next"`
	Results []dockerHubTag `json:"results"`
}

type dockerHubTag struct {
	Name          string `json:"name"`
	TagLastPushed string `json:"tag_last_pushed"`
}

// LatestTags returns tags for the policy's repository that match the configured
// tag pattern.
//
// Docker Hub serves tags ordered by last_updated, which is NOT semver order.
// For repositories with thousands of tags (harmony, bsc, klaytn) the genuine
// latest release can live many pages deep, so reading only the first page hides
// it and versioncheck reports a stale "latest". We therefore follow the
// response's "next" link, accumulating matching tags across pages until we hold
// a generous surplus (roughly maxResults*4), the pages run out, or we hit
// dockerHubMaxPages. Because ordering is not semver, we collect generously
// rather than trusting the first N — the caller picks the semver max (IsNewer).
func (c *dockerHubClient) LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, maxResults int) ([]TagEntry, error) {
	owner, repo := splitRepository(policy.Repository)

	pattern, err := regexp.Compile(policy.TagPattern)
	if err != nil {
		return nil, fmt.Errorf("compile tag pattern %q: %w", policy.TagPattern, err)
	}

	// Collect a surplus so the semver max is virtually guaranteed to be present
	// even though pages arrive in last_updated (not semver) order.
	target := maxResults * 4
	if target < maxResults {
		target = maxResults
	}

	next := fmt.Sprintf("%s/%s/%s/tags?page_size=%d&ordering=last_updated",
		dockerHubAPIBase, owner, repo, dockerHubPageSize)

	entries := make([]TagEntry, 0, target)

	for page := 0; page < dockerHubMaxPages && next != ""; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		payload, err := c.fetchPage(ctx, next, owner, repo)
		if err != nil {
			return nil, err
		}

		for _, t := range payload.Results {
			if !pattern.MatchString(t.Name) {
				continue
			}
			entries = append(entries, TagEntry{
				Tag:         t.Name,
				PublishedAt: t.TagLastPushed,
			})
		}

		if target > 0 && len(entries) >= target {
			break
		}

		next = resolveDockerHubNext(next, payload.Next)
	}

	return entries, nil
}

// resolveDockerHubNext returns the URL of the next page to fetch. Docker Hub
// emits an absolute URL in "next"; we follow it as-is. A relative link is
// resolved against the URL of the page we just fetched. An empty or unparseable
// link stops the walk by returning "".
func resolveDockerHubNext(current, next string) string {
	if next == "" {
		return ""
	}
	nextURL, err := url.Parse(next)
	if err != nil {
		return ""
	}
	if nextURL.IsAbs() {
		return nextURL.String()
	}
	base, err := url.Parse(current)
	if err != nil {
		return ""
	}
	return base.ResolveReference(nextURL).String()
}

func (c *dockerHubClient) fetchPage(ctx context.Context, pageURL, owner, repo string) (*dockerHubTagsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker hub request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker hub returned %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var result dockerHubTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

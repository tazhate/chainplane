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
	"strings"
	"time"

	"github.com/tazhate/chainplane/internal/adapters"
)

// ociClient speaks the Docker Distribution / OCI v2 tag-listing protocol.
// It handles public OCI registries with two flavors:
//
//   - Google Artifact Registry (us-docker.pkg.dev): public repos are
//     anonymous-readable; tags are returned in a non-standard nested shape
//     under {"manifest": {digest: {"tag": [...]}}}.
//   - Standard OCI registries (e.g. public.ecr.aws): require a bearer token
//     fetched from /token; tags are returned as {"tags": [...]}.
type ociClient struct {
	host string
	http *http.Client
}

const garHost = "us-docker.pkg.dev"

func (c *ociClient) httpClient() *http.Client {
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	return c.http
}

// LatestTags fetches tags from the configured OCI registry and filters them by
// policy.TagPattern. The maxResults cap applies after filtering.
func (c *ociClient) LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, maxResults int) ([]TagEntry, error) {
	repo := policy.Repository
	if strings.HasPrefix(repo, c.host+"/") {
		repo = strings.TrimPrefix(repo, c.host+"/")
	}

	var (
		tags []string
		err  error
	)
	if c.host == garHost {
		tags, err = c.fetchGARTags(ctx, repo)
	} else {
		tags, err = c.fetchStandardTags(ctx, repo, maxResults)
	}
	if err != nil {
		return nil, err
	}

	pattern, perr := regexp.Compile(policy.TagPattern)
	if perr != nil {
		return nil, fmt.Errorf("compile tag pattern %q: %w", policy.TagPattern, perr)
	}

	entries := make([]TagEntry, 0, maxResults)
	for _, tag := range tags {
		if !pattern.MatchString(tag) {
			continue
		}
		entries = append(entries, TagEntry{Tag: tag})
		if len(entries) >= maxResults {
			break
		}
	}
	return entries, nil
}

// fetchGARTags reads all tags from a GAR repository. GAR ignores ?n= pagination
// and dumps every digest's tag list in one response, so we accept the full
// payload (typically a few MB for active op-stack repos) and flatten it.
func (c *ociClient) fetchGARTags(ctx context.Context, repo string) ([]string, error) {
	tagsURL := fmt.Sprintf("https://%s/v2/%s/tags/list", c.host, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build GAR tags request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GAR tags request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GAR returned %d for %s", resp.StatusCode, repo)
	}

	var result garTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode GAR tags response: %w", err)
	}

	out := make([]string, 0, len(result.Manifest))
	for _, m := range result.Manifest {
		out = append(out, m.Tag...)
	}
	return out, nil
}

// fetchStandardTags uses the Docker Distribution token flow: anonymous bearer
// token from /token, then /v2/{repo}/tags/list with the token attached.
func (c *ociClient) fetchStandardTags(ctx context.Context, repo string, maxResults int) ([]string, error) {
	token, err := c.getToken(ctx, repo)
	if err != nil {
		return nil, err
	}

	fetchN := maxResults * 10
	if fetchN < 100 {
		fetchN = 100
	}

	tagsURL := fmt.Sprintf("https://%s/v2/%s/tags/list?n=%d", c.host, repo, fetchN)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build tags request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tags request to %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d for %s", c.host, resp.StatusCode, repo)
	}

	var result ociTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode tags response: %w", err)
	}
	return result.Tags, nil
}

type ociTokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"` // ECR Public uses this field
}

// getToken obtains an anonymous pull token via the standard OAuth2 scope URL.
// ECR Public follows the standard www-authenticate/token pattern.
func (c *ociClient) getToken(ctx context.Context, repo string) (string, error) {
	tokenURL := fmt.Sprintf("https://%s/token?scope=repository:%s:pull&service=%s",
		c.host, url.QueryEscape(repo), c.host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch token from %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint %s returned %d", c.host, resp.StatusCode)
	}

	var tr ociTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tr.Token != "" {
		return tr.Token, nil
	}
	return tr.AccessToken, nil
}

type ociTagsResponse struct {
	Tags []string `json:"tags"`
}

// garTagsResponse is the non-standard envelope returned by Google Artifact
// Registry. Tags are buried inside per-digest manifest entries.
type garTagsResponse struct {
	Manifest map[string]garManifestEntry `json:"manifest"`
}

type garManifestEntry struct {
	Tag []string `json:"tag"`
}
